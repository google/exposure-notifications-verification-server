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

// This server implements the device facing APIs for exchanging verification codes
// for tokens and tokens for certificates.
package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
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

	// Setup cacher
	// TODO(sethvargo): switch to HMAC
	cacher, err := cache.CacherFor(ctx, &config.Cache, cache.MultiKeyFunc(
		cache.HashKeyFunc(sha1.New), cache.PrefixKeyFunc("apiserver:")))
	if err != nil {
		return fmt.Errorf("failed to create cacher: %w", err)
	}
	defer cacher.Close()

	// Setup database
	db, err := config.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.OpenWithCacher(ctx, cacher); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Setup signers
	tokenSigner, err := keys.KeyManagerFor(ctx, &config.TokenSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create token key manager: %w", err)
	}
	certificateSigner, err := keys.KeyManagerFor(ctx, &config.CertificateSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create certificate key manager: %w", err)
	}

	// Create the router
	r := mux.NewRouter()

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close()

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.APIKeyFunc(ctx, "apiserver", db),
		limitware.AllowOnError(false))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Install HSTS headers in production
	if !config.DevMode {
		addHSTS := middleware.AddHSTS(ctx)
		r.Use(addHSTS)
	}

	// Create the renderer
	h, err := render.New(ctx, "", config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	r.Handle("/health", controller.HandleHealthz(ctx, &config.Database, h)).Methods("GET")

	// Setup API auth
	requireAPIKey := middleware.RequireAPIKey(ctx, cacher, db, h, []database.APIUserType{
		database.APIUserTypeDevice,
	})

	// Install the rate limiting first. In this case, we want to limit by key
	// first to reduce the chance of a database lookup.
	r.Use(rateLimit)

	// Install the APIKey Auth Middleware
	r.Use(requireAPIKey)

	// POST /api/verify
	verifyChaff := chaff.New()
	defer verifyChaff.Close()
	verifyapiController, err := verifyapi.New(ctx, config, db, h, tokenSigner)
	if err != nil {
		return fmt.Errorf("failed to create verify api controller: %w", err)
	}
	r.Handle("/api/verify", handleChaff(verifyChaff, verifyapiController.HandleVerify())).Methods("POST")

	// POST /api/certificate
	certChaff := chaff.New()
	defer certChaff.Close()
	certapiController, err := certapi.New(ctx, config, db, h, certificateSigner)
	if err != nil {
		return fmt.Errorf("failed to create certapi controller: %w", err)
	}
	r.Handle("/api/certificate", handleChaff(certChaff, certapiController.HandleCertificate())).Methods("POST")

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

func handleChaff(tracker *chaff.Tracker, next http.Handler) http.Handler {
	return tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"), next)
}
