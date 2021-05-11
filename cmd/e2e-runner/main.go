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
	"os/signal"
	"syscall"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

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
	cfg, err := config.NewE2ERunnerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process e2e-runner config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oe, err := observability.NewFromEnv(cfg.Observability)
	if err != nil {
		return fmt.Errorf("unable to create ObservabilityExporter provider: %w", err)
	}
	if err := oe.StartExporter(ctx); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	ctx, obs := middleware.WithObservability(ctx)
	logger.Infow("observability exporter", "config", cfg.Observability)

	db, err := cfg.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the renderer
	h, err := render.New(ctx, nil, cfg.DevMode)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	// Bootstrap the environment
	resp, err := envstest.Bootstrap(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to bootstrap testsuite: %w", err)
	}
	defer func() {
		if err := resp.Cleanup(); err != nil {
			logger.Errorw("failed to cleanup", "error", err)
		}
	}()

	// Verify that SMS is configured on the realm
	if !project.SkipE2ESMS {
		has, err := resp.Realm.HasSMSConfig(db)
		if err != nil {
			return fmt.Errorf("failed to check if realm has sms config: %w", err)
		}
		if !has {
			return fmt.Errorf("realm does not have sms config, configure it or set E2E_SKIP_SMS to continue")
		}
	}

	// Create the enx-redirect client if the URL was specified
	var enxRedirectClient *clients.ENXRedirectClient
	if u := cfg.ENXRedirectURL; u != "" {
		var err error
		enxRedirectClient, err = clients.NewENXRedirectClient(u,
			clients.WithTimeout(30*time.Second))
		if err != nil {
			return fmt.Errorf("failed to create enx-redirect client: %w", err)
		}
	}

	cfg.VerificationAdminAPIKey = resp.AdminAPIKey
	cfg.VerificationAPIServerKey = resp.DeviceAPIKey

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

	// Recovery injection
	recovery := middleware.Recovery(h)
	r.Use(recovery)

	r.Handle("/default", handleDefault(cfg, db, h))
	r.Handle("/revise", handleRevise(cfg, db, h))
	r.Handle("/user-report", handleUserReport(cfg, db, h))
	r.Handle("/enx-redirect", handleENXRedirect(enxRedirectClient, h))

	mux := http.Handler(r)
	if cfg.DevMode {
		// Also log requests in local dev.
		mux = handlers.LoggingHandler(os.Stdout, r)
	}

	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, mux)
}

// handleDefault handles the default end-to-end scenario.
func handleDefault(cfg *config.E2ERunnerConfig, db *database.Database, h *render.Renderer) http.Handler {
	c := *cfg
	c.DoRevise = false
	c.DoUserReport = false
	return handleEndToEnd(&c, db, h, mDefaultSuccess)
}

// handleRevise runs the end-to-end runner with revision tokens.
func handleRevise(cfg *config.E2ERunnerConfig, db *database.Database, h *render.Renderer) http.Handler {
	c := *cfg
	c.DoRevise = true
	c.DoUserReport = false
	return handleEndToEnd(&c, db, h, mRevisionSuccess)
}

// handleUserReport runs the end-to-end runner initiated by a user-report API request.
func handleUserReport(cfg *config.E2ERunnerConfig, db *database.Database, h *render.Renderer) http.Handler {
	c := *cfg
	c.DoRevise = false
	c.DoUserReport = true
	return handleEndToEnd(&c, db, h, mUserReportSuccess)
}

// handleEndToEnd handles the common end-to-end scenario. m is incremented iff
// the run succeeds.
func handleEndToEnd(cfg *config.E2ERunnerConfig, db *database.Database, h *render.Renderer, m *stats.Int64Measure) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		if cfg.DoUserReport {
			if err := db.DeleteUserReport(project.TestPhoneNumber); err != nil {
				logger.Errorw("error deleting previous user report for test phone number", "error", err)
				h.RenderJSON(w, http.StatusInternalServerError, err)
				return
			}
		}

		if err := clients.RunEndToEnd(ctx, cfg); err != nil {
			logger.Errorw("failure", "error", err)
			h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, m.M(1))
		h.RenderJSON(w, http.StatusOK, nil)
	})
}

// handleENXRedirect handles tests for the redirector service.
func handleENXRedirect(client *clients.ENXRedirectClient, h *render.Renderer) http.Handler {
	// If the client doesn't exist, it means the host was not provided.
	if client == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.RenderJSON(w, http.StatusOK, nil)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		if err := client.RunE2E(ctx); err != nil {
			logger.Errorw("failure", "error", err)
			h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mRedirectSuccess.M(1))
		h.RenderJSON(w, http.StatusOK, nil)
	})
}
