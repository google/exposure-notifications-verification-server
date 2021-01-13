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

// This is a server that invokes end-to-end tests.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().
		With("build_id", buildinfo.BuildID).
		With("build_tag", buildinfo.BuildTag)
	ctx = logging.WithLogger(ctx, logger)

	defer func() {
		done()
		if r := recover(); r != nil {
			logger.Fatalw("application panic", "panic", r)
		}
	}()

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	// load configs
	e2eConfig, err := config.NewE2ERunnerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process e2e-runner config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oe, err := observability.NewFromEnv(e2eConfig.Observability)
	if err != nil {
		return fmt.Errorf("unable to create ObservabilityExporter provider: %w", err)
	}
	if err := oe.StartExporter(ctx); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	ctx, obs := middleware.WithObservability(ctx)
	logger.Infow("observability exporter", "config", e2eConfig.Observability)

	db, err := e2eConfig.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the renderer
	h, err := render.New(ctx, "", e2eConfig.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	resp, err := envstest.Bootstrap(db)
	if err != nil {
		return fmt.Errorf("failed to bootstrap testsuite: %w", err)
	}
	defer func() {
		if err := resp.Cleanup(); err != nil {
			logger.Errorw("failed to cleanup", "error", err)
		}
	}()

	e2eConfig.TestConfig.VerificationAdminAPIKey = resp.AdminAPIKey
	e2eConfig.TestConfig.VerificationAPIServerKey = resp.DeviceAPIKey

	// Create the router
	r := mux.NewRouter()

	// Common observability context
	r.Use(obs)

	// Request ID injection
	populateRequestID := middleware.PopulateRequestID(h)
	r.Use(populateRequestID)

	// Logger injection
	populateLogger := middleware.PopulateLogger(logger)
	r.Use(populateLogger)

	r.HandleFunc("/default", defaultHandler(ctx, e2eConfig.TestConfig))
	r.HandleFunc("/revise", reviseHandler(ctx, e2eConfig.TestConfig))

	mux := http.Handler(r)
	if e2eConfig.DevMode {
		// Also log requests in local dev.
		mux = handlers.LoggingHandler(os.Stdout, r)
	}

	srv, err := server.New(e2eConfig.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", e2eConfig.Port)
	return srv.ServeHTTPHandler(ctx, mux)
}

// Config is passed by value so that each http handler has a separate copy (since they are changing one of the)
// config elements. Previous versions of those code had a race condition where the "DoRevise" status
// could be changed while a handler was executing.
func defaultHandler(ctx context.Context, config config.E2ETestConfig) func(http.ResponseWriter, *http.Request) {
	logger := logging.FromContext(ctx)
	c := &config
	c.DoRevise = false
	return func(w http.ResponseWriter, r *http.Request) {
		if err := clients.RunEndToEnd(ctx, c); err != nil {
			logger.Errorw("could not run default end to end", "error", err)
			http.Error(w, "failed (check server logs for more details): "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, "ok")
	}
}

func reviseHandler(ctx context.Context, config config.E2ETestConfig) func(http.ResponseWriter, *http.Request) {
	logger := logging.FromContext(ctx)
	c := &config
	c.DoRevise = true
	return func(w http.ResponseWriter, r *http.Request) {
		if err := clients.RunEndToEnd(ctx, c); err != nil {
			logger.Errorw("could not run revise end to end", "error", err)
			http.Error(w, "failed (check server logs for more details): "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, "ok")
	}
}
