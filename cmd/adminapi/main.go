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

// This server implements the admin facing APIs for issuing diagnosis codes
// and checking the status of previously issued codes.
package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codestatus"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

	config, err := config.NewAdminAPIServerConfig(ctx)
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
		cache.HashKeyFunc(sha1.New), cache.PrefixKeyFunc("adminapi:")))
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

	// Create the router
	r := mux.NewRouter()

	// Rate limiting
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &config.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close()

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.APIKeyFunc(ctx, "adminapi", db),
		limitware.AllowOnError(false))
	if err != nil {
		return fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Install common security headers
	r.Use(middleware.SecureHeaders(ctx, config.DevMode, "json"))

	// Create the renderer
	h, err := render.New(ctx, "", config.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Install the rate limiting first. In this case, we want to limit by key
	// first to reduce the chance of a database lookup.
	r.Use(rateLimit)

	r.Handle("/health", controller.HandleHealthz(ctx, &config.Database, h)).Methods("GET")
	{
		sub := r.PathPrefix("/api").Subrouter()

		// Setup API auth
		requireAPIKey := middleware.RequireAPIKey(ctx, cacher, db, h, []database.APIUserType{
			database.APIUserTypeDevice,
		})
		// Install the APIKey Auth Middleware
		sub.Use(requireAPIKey)

		issueapiController, err := issueapi.New(ctx, config, db, h)
		if err != nil {
			return fmt.Errorf("issueapi.New: %w", err)
		}
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")

		codeStatusController := codestatus.NewAPI(ctx, config, db, h)
		sub.Handle("/checkcodestatus", codeStatusController.HandleCheckCodeStatus()).Methods("POST")
		sub.Handle("/expirecode", codeStatusController.HandleExpireAPI()).Methods("POST")
	}

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}
