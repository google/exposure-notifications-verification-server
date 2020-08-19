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
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codestatus"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realm"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	firebase "firebase.google.com/go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	debug, _ := strconv.ParseBool(os.Getenv("LOG_DEBUG"))
	logger := logging.NewLogger(debug)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

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
	if err := oe.StartExporter(); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	logger.Infow("observability exporter", "config", oeConfig)

	// Setup sessions
	sessions := sessions.NewCookieStore(config.CookieKeys.AsBytes()...)
	sessions.Options.Path = "/"
	sessions.Options.Domain = config.CookieDomain
	sessions.Options.MaxAge = int(config.SessionDuration.Seconds())
	sessions.Options.Secure = !config.DevMode
	sessions.Options.SameSite = http.SameSiteStrictMode

	// Setup database
	db, err := config.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
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
	h, err := render.New(ctx, config.AssetsPath, config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close()

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.UserEmailKeyFunc(ctx, "server"),
		limitware.AllowOnError(false))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}

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
	rateLimit := httplimiter.Handle

	{
		sub := r.PathPrefix("").Subrouter()
		sub.Use(rateLimit)

		indexController := index.New(ctx, config, h)
		sub.Handle("/", indexController.HandleIndex()).Methods("GET")
		sub.Handle("/healthz", controller.HandleHealthz(ctx, h, &config.Database)).Methods("GET")

		// Session handling
		sessionController := session.New(ctx, auth, config, db, h)
		sub.Handle("/signout", sessionController.HandleDelete()).Methods("GET")
		sub.Handle("/session", sessionController.HandleCreate()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/login").Subrouter()
		sub.Use(rateLimit)

		loginController := login.New(ctx, config, h)
		sub.Handle("", loginController.HandleLogin()).Methods("GET")

		// Session handling
		// TODO(whaught): these are duplicated so they serve at this path for ajax
		sessionController := session.New(ctx, auth, config, db, h)
		sub.Handle("/session", sessionController.HandleCreate()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/realm").Subrouter()
		sub.Use(requireAuth)
		sub.Use(rateLimit)

		// Realms - list and select.
		realmController := realm.New(ctx, config, db, h)
		sub.Handle("", realmController.HandleIndex()).Methods("GET")
		sub.Handle("/select", realmController.HandleSelect()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/home").Subrouter()
		sub.Use(requireAuth)
		sub.Use(requireRealm)
		sub.Use(rateLimit)

		homeController := home.New(ctx, config, db, h)
		sub.Handle("", homeController.HandleHome()).Methods("GET")

		// API for creating new verification codes. Called via AJAX.
		issueapiController := issueapi.New(ctx, config, db, h)
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/code").Subrouter()
		sub.Use(requireAuth)
		sub.Use(requireRealm)
		sub.Use(rateLimit)

		codeStatusController := codestatus.NewServer(ctx, config, db, h)
		sub.Handle("/status", codeStatusController.HandleIndex()).Methods("GET")
		sub.Handle("/show", codeStatusController.HandleShow()).Methods("POST")
	}

	// apikeys
	{
		sub := r.PathPrefix("/apikeys").Subrouter()
		sub.Use(requireAuth)
		sub.Use(requireRealm)
		sub.Use(requireAdmin)
		sub.Use(rateLimit)

		apikeyController := apikey.New(ctx, config, db, h)
		sub.Handle("", apikeyController.HandleIndex()).Methods("GET")
		sub.Handle("", apikeyController.HandleCreate()).Methods("POST")
		sub.Handle("/new", apikeyController.HandleCreate()).Methods("GET")
		sub.Handle("/{id}/edit", apikeyController.HandleUpdate()).Methods("GET")
		sub.Handle("/{id}", apikeyController.HandleShow()).Methods("GET")
		sub.Handle("/{id}", apikeyController.HandleUpdate()).Methods("PATCH")
		sub.Handle("/{id}/disable", apikeyController.HandleDisable()).Methods("PATCH")
		sub.Handle("/{id}/enable", apikeyController.HandleEnable()).Methods("PATCH")
	}

	// users
	{
		userSub := r.PathPrefix("/users").Subrouter()
		userSub.Use(requireAuth)
		userSub.Use(requireRealm)
		userSub.Use(requireAdmin)
		userSub.Use(rateLimit)

		userController := user.New(ctx, config, db, h)
		userSub.Handle("", userController.HandleIndex()).Methods("GET")
		userSub.Handle("", userController.HandleCreate()).Methods("POST")
		userSub.Handle("/new", userController.HandleCreate()).Methods("GET")
		userSub.Handle("/{id}/edit", userController.HandleUpdate()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleShow()).Methods("GET")
		userSub.Handle("/{id}", userController.HandleUpdate()).Methods("PATCH")
		userSub.Handle("/{id}", userController.HandleDelete()).Methods("DELETE")
	}

	// realms
	{
		realmSub := r.PathPrefix("/realm/settings").Subrouter()
		realmSub.Use(requireAuth)
		realmSub.Use(requireRealm)
		realmSub.Use(requireAdmin)
		realmSub.Use(rateLimit)

		realmadminController := realmadmin.New(ctx, config, db, h)
		realmSub.Handle("", realmadminController.HandleIndex()).Methods("GET")
		realmSub.Handle("/save", realmadminController.HandleSave()).Methods("POST")
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod(ctx)(r))

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, mux))
}
