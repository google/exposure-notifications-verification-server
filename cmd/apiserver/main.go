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
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
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
	logger = logger.With("build_id", buildinfo.BuildID)
	logger = logger.With("build_tag", buildinfo.BuildTag)

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

	cfg, err := config.NewAPIServerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oeConfig := cfg.ObservabilityExporterConfig()
	oe, err := observability.NewFromEnv(oeConfig)
	if err != nil {
		return fmt.Errorf("unable to create ObservabilityExporter provider: %w", err)
	}
	if err := oe.StartExporter(); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	logger.Infow("observability exporter", "config", oeConfig)

	// Setup cacher
	cacher, err := cache.CacherFor(ctx, &cfg.Cache, cache.HMACKeyFunc(sha1.New, cfg.Cache.HMACKey))
	if err != nil {
		return fmt.Errorf("failed to create cacher: %w", err)
	}
	defer cacher.Close()

	// Setup database
	db, err := cfg.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.OpenWithCacher(ctx, cacher); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Setup signers
	tokenSigner, err := keys.KeyManagerFor(ctx, &cfg.TokenSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create token key manager: %w", err)
	}
	certificateSigner, err := keys.KeyManagerFor(ctx, &cfg.CertificateSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create certificate key manager: %w", err)
	}

	// Create the router
	r := mux.NewRouter()

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &cfg.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close(ctx)

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.APIKeyFunc(ctx, db, "apiserver:ratelimit:", cfg.RateLimit.HMACKey),
		limitware.AllowOnError(false))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Install common security headers
	r.Use(middleware.SecureHeaders(cfg.DevMode, "json"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug()
	r.Use(processDebug)

	// Create the renderer
	h, err := render.New(ctx, "", cfg.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Request ID injection
	populateRequestID := middleware.PopulateRequestID(h)
	r.Use(populateRequestID)

	// Logger injection
	populateLogger := middleware.PopulateLogger(logger)
	r.Use(populateLogger)

	// Install the rate limiting first. In this case, we want to limit by key
	// first to reduce the chance of a database lookup.
	r.Use(rateLimit)

	// Other common middlewares
	requireAPIKey := middleware.RequireAPIKey(cacher, db, h, []database.APIKeyType{
		database.APIKeyTypeDevice,
	})
	processFirewall := middleware.ProcessFirewall(h, "apiserver")

	r.Handle("/health", controller.HandleHealthz(ctx, &cfg.Database, h)).Methods("GET")

	{
		sub := r.PathPrefix("/api").Subrouter()
		sub.Use(requireAPIKey)
		sub.Use(processFirewall)

		chaffDet := chaff.HeaderDetector("X-Chaff")

		// POST /api/verify
		verifyChaff, err := chaff.NewTracker(chaff.NewJSONResponder(encodeVerifyReponse), chaff.DefaultCapacity)
		if err != nil {
			return fmt.Errorf("error creating chaffer: %v", err)
		}

		defer verifyChaff.Close()
		verifyapiController, err := verifyapi.New(ctx, cfg, db, h, tokenSigner)
		if err != nil {
			return fmt.Errorf("failed to create verify api controller: %w", err)
		}
		sub.Handle("/verify", verifyChaff.HandleTrack(chaffDet, verifyapiController.HandleVerify())).Methods("POST")

		// POST /api/certificate
		certChaff, err := chaff.NewTracker(chaff.NewJSONResponder(encodeCertificateResponse), chaff.DefaultCapacity)
		if err != nil {
			return fmt.Errorf("error creating chaffer: %v", err)
		}
		defer certChaff.Close()
		certapiController, err := certapi.New(ctx, cfg, db, cacher, certificateSigner, h)
		if err != nil {
			return fmt.Errorf("failed to create certapi controller: %w", err)
		}
		sub.Handle("/certificate", certChaff.HandleTrack(chaffDet, certapiController.HandleCertificate())).Methods("POST")
	}

	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

// makePadFromChaff makes a Padding structure from chaff data.
// Note, the random chaff data will be longer than necessary, so we shorten it.
func makePadFromChaff(s string) api.Padding {
	return api.Padding(s)
}

func encodeVerifyReponse(s string) interface{} {
	return api.VerifyCodeResponse{Padding: makePadFromChaff(s)}
}

func encodeCertificateResponse(s string) interface{} {
	return api.VerificationCertificateResponse{Padding: makePadFromChaff(s)}
}
