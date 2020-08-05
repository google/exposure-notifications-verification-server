// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realm"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/server"

	firebase "firebase.google.com/go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	err := realMain(ctx)
	done()

	logger := logging.FromContext(ctx)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	config, err := config.NewServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oeConfig := config.ObservabilityExporterConfig()
	oe, err := observability.NewFromEnv(ctx, oeConfig)
	if err != nil {
		return fmt.Errorf("unable to create ObservabilityExporter provider: %w", err)
	}
	if err := oe.InitExportOnce(); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	logger.Infow("observability exporter", "config", oeConfig)

	// Setup sessions
	sessions := sessions.NewCookieStore(config.CookieKeys.AsBytes()...)
	sessions.Options.Path = "/"
	sessions.Options.Domain = config.CookieDomain
	sessions.Options.MaxAge = int(config.SessionDuration.Seconds())
	sessions.Options.Secure = !config.DevMode
	sessions.Options.SameSite = http.SameSiteStrictMode

	// Setup database
	db, err := config.Database.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Setup firebase
	app, err := firebase.NewApp(ctx, config.FirebaseConfig())
	if err != nil {
		return fmt.Errorf("failed to setup firebase: %w", err)
	}
	auth, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("failed to configure firebase: %w", err)
	}

	// Create the router
	r := mux.NewRouter()

	// Inject template middleware - this needs to be first because other
	// middlewares may add data to the template map.
	populateTemplateVariables := middleware.PopulateTemplateVariables(ctx, config)
	r.Use(populateTemplateVariables)

	// Create the renderer
	glob := filepath.Join(config.AssetsPath, "*")
	h, err := render.New(ctx, glob, config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Setup rate limiting
	store, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer store.Close()

	httplimiter, err := httplimit.NewMiddleware(store, userEmailKeyFunc())
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	r.Use(httplimiter.Handle)

	// Install the CSRF protection middleware.
	configureCSRF := middleware.ConfigureCSRF(ctx, config, h)
	r.Use(configureCSRF)

	// Sessions
	requireSession := middleware.RequireSession(ctx, sessions, h)
	r.Use(requireSession)

	// Create common middleware
	requireAuth := middleware.RequireAuth(ctx, auth, db, h, config.SessionDuration)
	requireAdmin := middleware.RequireRealmAdmin(ctx, h)
	requireRealm := middleware.RequireRealm(ctx, db, h)

	// Install the handlers that don't require authentication first on the main router.
	indexController := index.New(ctx, config, h)
	r.Handle("/", indexController.HandleIndex()).Methods("GET")

	// Session handling
	sessionController := session.New(ctx, auth, config, db, h)
	r.Handle("/signout", sessionController.HandleDelete()).Methods("GET")
	r.Handle("/session", sessionController.HandleCreate()).Methods("POST")

	{
		sub := r.PathPrefix("/realm").Subrouter()
		sub.Use(requireAuth)

		// Realms - list and select.
		realmController := realm.New(ctx, config, db, h)
		sub.Handle("", realmController.HandleIndex()).Methods("GET")
		sub.Handle("/select", realmController.HandleSelect()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/home").Subrouter()
		sub.Use(requireAuth)
		sub.Use(requireRealm)

		homeController := home.New(ctx, config, db, h)
		sub.Handle("", homeController.HandleHome()).Methods("GET")

		// API for creating new verification codes. Called via AJAX.
		issueapiController := issueapi.New(ctx, config, db, h)
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")
	}

	// apikeys
	{
		sub := r.PathPrefix("/apikeys").Subrouter()
		sub.Use(requireAuth)
		sub.Use(requireRealm)
		sub.Use(requireAdmin)

		apikeyController := apikey.New(ctx, config, db, h)
		sub.Handle("", apikeyController.HandleIndex()).Methods("GET")
		sub.Handle("/create", apikeyController.HandleCreate()).Methods("POST")
	}

	// users
	{
		userSub := r.PathPrefix("/users").Subrouter()
		userSub.Use(requireAuth)
		userSub.Use(requireRealm)
		userSub.Use(requireAdmin)

		userController := user.New(ctx, config, db, h)
		userSub.Handle("", userController.HandleIndex()).Methods("GET")
		userSub.Handle("/create", userController.HandleCreate()).Methods("POST")
		userSub.Handle("/delete/{email}", userController.HandleDelete()).Methods("POST")
	}

	// realms
	{
		realmSub := r.PathPrefix("/realm/settings").Subrouter()
		realmSub.Use(requireAuth)
		realmSub.Use(requireRealm)
		realmSub.Use(requireAdmin)

		realmadminController := realmadmin.New(ctx, config, db, h)
		realmSub.Handle("", realmadminController.HandleIndex()).Methods("GET")
		realmSub.Handle("/save", realmadminController.HandleSave()).Methods("POST")
	}

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

func userEmailKeyFunc() httplimit.KeyFunc {
	ipKeyFunc := httplimit.IPKeyFunc("X-Forwarded-For")

	return func(r *http.Request) (string, error) {
		user := controller.UserFromContext(r.Context())
		if user != nil && user.Email != "" {
			dig := sha1.Sum([]byte(user.Email))
			return fmt.Sprintf("%x", dig), nil
		}

		return ipKeyFunc(r)
	}
}
