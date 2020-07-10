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
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	firebase "firebase.google.com/go"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	csrfctl "github.com/google/exposure-notifications-verification-server/pkg/controller/csrf"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/home"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/index"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/session"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/signout"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	ctx := context.Background()
	config, err := config.NewServerConfig(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	// Setup database
	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer db.Close()

	// Setup firebase
	app, err := firebase.NewApp(ctx, config.FirebaseConfig())
	if err != nil {
		log.Fatalf("failed to setup firebase: %v", err)
	}
	auth, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("failed to configure firebase auth: %v", err)
	}

	// Create our HTML renderer.
	renderHTML := render.LoadHTMLGlob(config.AssetsPath + "/*")

	r := mux.NewRouter()

	// Setup rate limiter
	store, err := memorystore.New(&memorystore.Config{
		Tokens:   config.RateLimit,
		Interval: 1 * time.Minute,
	})
	if err != nil {
		log.Fatalf("failed to create limiter: %v", err)
	}
	defer store.Stop()

	httplimiter, err := httplimit.NewMiddleware(store, httplimit.IPKeyFunc("X-Forwarded-For"))
	if err != nil {
		log.Fatalf("failed to create limiter middleware: %v", err)
	}
	r.Use(httplimiter.Handle)

	r.Use(html.New(config).Handle)
	// install the CSRF protection middleware.
	csrfAuthKey, err := config.CSRFKey()
	if err != nil {
		log.Fatalf("unable to configure CSRF protection: %v", err)
	}
	// TODO(mikehelmick) - there are many more configuration options for CSRF protection.
	r.Use(csrf.Protect(
		csrfAuthKey,
		csrf.Secure(!config.DevMode),
		csrf.SameSite(csrf.SameSiteLaxMode),
		csrf.ErrorHandler(http.HandlerFunc(csrfctl.ErrorHandler))))
	r.Use(middleware.FlashHandler)

	// Install the handlers that don't require authentication first on the main router.
	r.Handle("/", index.New(config, renderHTML)).Methods("GET")
	r.Handle("/signout", signout.New(config, db, renderHTML)).Methods("GET")
	r.Handle("/session", session.New(ctx, config, auth, db)).Methods("POST")

	{
		sub := r.PathPrefix("/home").Subrouter()
		sub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		sub.Handle("", home.New(ctx, config, db, renderHTML)).Methods("GET")

		// API for creating new verification codes. Called via AJAX.
		sub.Handle("/issue", issueapi.New(ctx, config, db)).Methods("POST")

		// API for obtaining a CSRF token before calling /issue
		// Installed in this context, it requires authentication.
		sub.Handle("/csrf", csrfctl.NewCSRFAPI()).Methods("GET")
	}

	// Admin pages, requires admin auth
	{
		sub := r.PathPrefix("/apikeys").Subrouter()
		sub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		sub.Use(middleware.RequireAdmin(ctx).Handle)

		sub.Handle("", apikey.NewListController(ctx, config, db, renderHTML)).Methods("GET")
		sub.Handle("/create", apikey.NewSaveController(ctx, config, db)).Methods("POST")

		userSub := r.PathPrefix("/users").Subrouter()
		userSub.Use(middleware.RequireAuth(ctx, auth, db, config.SessionCookieDuration).Handle)
		userSub.Use(middleware.RequireAdmin(ctx).Handle)

		userSub.Handle("", user.NewListController(ctx, config, db, renderHTML)).Methods("GET")
		userSub.Handle("/create", user.NewSaveController(ctx, config, db)).Methods("POST")
		userSub.Handle("/delete/{email}", user.NewDeleteController(ctx, config, db)).Methods("POST")
	}

	srv := &http.Server{
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
		Addr:    "0.0.0.0:" + strconv.Itoa(config.Port),
	}
	log.Printf("Listening on :%v", config.Port)
	log.Fatal(srv.ListenAndServe())
}
