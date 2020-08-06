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
	"os"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/mikehelmick/go-chaff"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-signalcontext"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

	config, err := config.NewAPIServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup database
	db, err := config.Database.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Setup signer
	signer, err := gcpkms.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to crate key manager: %w", err)
	}

	// Create the router
	r := mux.NewRouter()

	// Setup rate limiter
	store, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer store.Close()

	httplimiter, err := httplimit.NewMiddleware(store, apiKeyFunc())
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	r.Use(httplimiter.Handle)

	// Create the renderer
	h, err := render.New(ctx, "", config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	r.Handle("/healthz", controller.HandleHealthz(h, &config.Database)).Methods("GET")

	// Setup API auth
	apiKeyCache, err := cache.New(config.APIKeyCacheDuration)
	if err != nil {
		return fmt.Errorf("failed to create apikey cache: %w", err)
	}
	requireAPIKey := middleware.RequireAPIKey(ctx, apiKeyCache, db, h, []database.APIUserType{
		database.APIUserTypeDevice,
	})

	// Install the APIKey Auth Middleware
	r.Use(requireAPIKey)

	publicKeyCache, err := cache.New(config.PublicKeyCacheDuration)
	if err != nil {
		return fmt.Errorf("failed to create publickey cache: %w", err)
	}

	// POST /api/verify
	verifyChaff := chaff.New()
	defer verifyChaff.Close()
	verifyapiController := verifyapi.New(ctx, config, db, h, signer)
	r.Handle("/api/verify", handleChaff(verifyChaff, verifyapiController.HandleVerify())).Methods("POST")

	// POST /api/certificate
	certChaff := chaff.New()
	defer certChaff.Close()
	certapiController := certapi.New(ctx, config, db, h, signer, publicKeyCache)
	r.Handle("/api/certificate", handleChaff(certChaff, certapiController.HandleCertificate())).Methods("POST")

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
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

func handleChaff(tracker *chaff.Tracker, next http.Handler) http.Handler {
	return tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"), next)
}
