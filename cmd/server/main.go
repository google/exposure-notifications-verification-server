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
	csrfctl "github.com/google/exposure-notifications-verification-server/pkg/controller/csrf"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realm"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/server"

	firebase "firebase.google.com/go"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

	// Create our HTML renderer
	glob := filepath.Join(config.AssetsPath, "*")
	h, err := render.New(ctx, glob, config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Inject template middleware
	// TODO(sethvargo): clean this up
	r.Use(html.New(config).Handle)

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
	csrfAuthKey, err := config.CSRFKey()
	if err != nil {
		return fmt.Errorf("failed to configure CSRF: %w", err)
	}
	// TODO(mikehelmick) - there are many more configuration options for CSRF protection.
	r.Use(csrf.Protect(
		csrfAuthKey,
		csrf.Secure(!config.DevMode),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.ErrorHandler(http.HandlerFunc(csrfctl.ErrorHandler))))
	r.Use(middleware.FlashHandler)

	// Install the handlers that don't require authentication first on the main router.
	indexController := index.New(ctx, config, h)
	r.Handle("/", indexController.HandleIndex()).Methods("GET")

	// Session handling
	sessionController := session.New(ctx, auth, config, db, h)
	r.Handle("/signout", sessionController.HandleDelete()).Methods("GET")
	r.Handle("/session", sessionController.HandleCreate()).Methods("POST")

	{
		sub := r.PathPrefix("/realm").Subrouter()
		sub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)

		// Realms - list and select.
		realmController := realm.New(ctx, config, db, h)
		sub.Handle("", realmController.HandleIndex()).Methods("GET")
		sub.Handle("/select", realmController.HandleSelect()).Methods("POST")
	}

	{
		sub := r.PathPrefix("/home").Subrouter()
		sub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		sub.Use(middleware.RequireRealm(ctx).Handle)

		homeController := home.New(ctx, config, db, h)
		sub.Handle("", homeController.HandleHome()).Methods("GET")

		// API for creating new verification codes. Called via AJAX.
		sub.Handle("/issue", issueapi.New(ctx, config, db)).Methods("POST")

		// API for obtaining a CSRF token before calling /issue
		// Installed in this context, it requires authentication.
		sub.Handle("/csrf", csrfctl.NewCSRFAPI()).Methods("GET")
	}

	// Realm Admin pages, requires realm admin.
	{
		sub := r.PathPrefix("/apikeys").Subrouter()
		sub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		sub.Use(middleware.RequireRealm(ctx).Handle)
		sub.Use(middleware.RequireRealmAdmin(ctx).Handle)

		apikeyController := apikey.New(ctx, config, db, h)
		sub.Handle("", apikeyController.HandleIndex()).Methods("GET")
		sub.Handle("/create", apikeyController.HandleCreate()).Methods("POST")

		userSub := r.PathPrefix("/users").Subrouter()
		userSub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		userSub.Use(middleware.RequireRealm(ctx).Handle)
		userSub.Use(middleware.RequireRealmAdmin(ctx).Handle)

		userController := user.New(ctx, config, db, h)
		userSub.Handle("", userController.HandleIndex()).Methods("GET")
		userSub.Handle("/create", userController.HandleCreate()).Methods("POST")
		userSub.Handle("/delete/{email}", userController.HandleDelete()).Methods("POST")

		realmSub := r.PathPrefix("/realm/settings").Subrouter()
		realmSub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		realmSub.Use(middleware.RequireRealm(ctx).Handle)
		realmSub.Use(middleware.RequireRealmAdmin(ctx).Handle)

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

		// If no API key was provided, default to limiting by IP.
		return ipKeyFunc(r)
	}
}
