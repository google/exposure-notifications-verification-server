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

// This server implements the device facing APIs for exchaning verification codes
// for tokens and tokens for certificates.
package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-limiter/memorystore"
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

	config, err := config.NewAdminAPIServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup database
	db, err := config.Database.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the router
	r := mux.NewRouter()

	// Setup chaff request handling
	tracker := chaff.New()
	defer tracker.Close()
	r.Use(tracker.Track)

	// Setup rate limiter
	store, err := memorystore.New(&memorystore.Config{
		Tokens:   config.RateLimit,
		Interval: 1 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer store.Close()

	httplimiter, err := httplimit.NewMiddleware(store, apiKeyFunc())
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	r.Use(httplimiter.Handle)

	// Setup API auth
	apiKeyCache, err := cache.New(config.APIKeyCacheDuration)
	if err != nil {
		return fmt.Errorf("failed to create apikey cache: %w", err)
	}
	// Install the APIKey Auth Middleware
	r.Use(middleware.APIKeyAuth(ctx, db, apiKeyCache, database.APIUserTypeAdmin).Handle)

	r.Handle("/api/issue", handleChaff(tracker, issueapi.New(ctx, config, db))).Methods("POST")

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, r)
}

func apiKeyFunc() httplimit.KeyFunc {
	ipKeyFunc := httplimit.IPKeyFunc("X-Forwarded-For")

	return func(r *http.Request) (string, error) {
		v := r.Header.Get("X-API-Key")
		if v != "" {
			dig := sha1.Sum([]byte(v))
			return fmt.Sprintf("%x", dig), nil
		}

		// If no API key was provided, default to limiting by IP.
		return ipKeyFunc(r)
	}
}

func handleChaff(onchaff http.Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Chaff") != "" {
			onchaff.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
