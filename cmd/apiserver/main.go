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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/gcpkms"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/mikehelmick/go-chaff"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-signalcontext"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLogger(true)
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

	config, err := config.NewAPIServerConfig(ctx)
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

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close()

	httplimiter, err := httplimit.NewMiddleware(limiterStore, limiterFunc(ctx, db))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Create the renderer
	h, err := render.New(ctx, "", config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	r.Handle("/healthz", controller.HandleHealthz(ctx, h, &config.Database)).Methods("GET")

	// Setup API auth
	apiKeyCache, err := cache.New(config.APIKeyCacheDuration)
	if err != nil {
		return fmt.Errorf("failed to create apikey cache: %w", err)
	}
	requireAPIKey := middleware.RequireAPIKey(ctx, apiKeyCache, db, h, []database.APIUserType{
		database.APIUserTypeDevice,
	})

	// Install the rate limiting first. In this case, we want to limit by key
	// first to reduce the chance of a database lookup.
	r.Use(rateLimit)

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

// limiterFunc is a custom rate limiter function. It limits by API key realm, if
// one exists, then by IP.
func limiterFunc(ctx context.Context, db *database.Database) httplimit.KeyFunc {
	logger := logging.FromContext(ctx).Named("ratelimit")

	return func(r *http.Request) (string, error) {
		// Procss the API key
		v := r.Header.Get("X-API-Key")
		if v != "" {
			// v2 API keys encode the realm
			_, realmID, err := db.VerifyAPIKeySignature(v)
			if err == nil {
				logger.Debugw("limiting by api key v2 realm", "realm", realmID)
				dig := sha1.Sum([]byte(fmt.Sprintf("%d", realmID)))
				return fmt.Sprintf("apiserver:realm:%x", dig), nil
			}

			// v1 API keys do not, fallback to the database
			app, err := db.FindAuthorizedAppByAPIKey(v)
			if err == nil && app != nil {
				logger.Debugw("limiting by api key v1 realm", "realm", app.RealmID)
				dig := sha1.Sum([]byte(fmt.Sprintf("%d", app.RealmID)))
				return fmt.Sprintf("apiserver:realm:%x", dig), nil
			}
		}

		// Get the remote addr
		ip := r.RemoteAddr

		// Check if x-forwarded-for exists, the load balancer sets this, and the
		// first entry is the real client IP
		xff := r.Header.Get("x-forwarded-for")
		if xff != "" {
			ip = strings.Split(xff, ",")[0]
		}

		logger.Debugw("limiting by ip", "ip", ip)
		dig := sha1.Sum([]byte(ip))
		return fmt.Sprintf("apiserver:ip:%x", dig), nil
	}
}

func handleChaff(tracker *chaff.Tracker, next http.Handler) http.Handler {
	return tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"), next)
}
