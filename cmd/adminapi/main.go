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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sethvargo/go-limiter/httplimit"
	"github.com/sethvargo/go-signalcontext"
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

	// Setup database
	db, err := config.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
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

	httplimiter, err := httplimit.NewMiddleware(limiterStore, limiterFunc(ctx))
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
		database.APIUserTypeAdmin,
	})

	// Install the APIKey Auth Middleware
	r.Use(requireAPIKey)
	r.Use(rateLimit)

	issueapiController := issueapi.New(ctx, config, db, h)
	r.Handle("/api/issue", issueapiController.HandleIssue()).Methods("POST")
	r.Handle("/api/checkcodestatus", issueapiController.HandleIssue()).Methods("POST")

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", config.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

// limiterFunc is a custom rate limiter function. It limits by realm (by API
// key, if one exists, then by IP.
func limiterFunc(ctx context.Context) httplimit.KeyFunc {
	logger := logging.FromContext(ctx).Named("ratelimit")

	return func(r *http.Request) (string, error) {
		ctx := r.Context()

		// See if a user exists on the context
		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp != nil && authApp.RealmID != 0 {
			logger.Debugw("limiting by authApp realm", "authApp", authApp.ID)
			dig := sha1.Sum([]byte(fmt.Sprintf("%d", authApp.RealmID)))
			return fmt.Sprintf("adminapi:realm:%x", dig), nil
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
		return fmt.Sprintf("adminapi:ip:%x", dig), nil
	}
}
