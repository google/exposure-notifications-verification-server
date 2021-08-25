// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"crypto/sha1"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/gorilla/handlers"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
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

	cfg, err := config.NewRedirectConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup monitoring
	logger.Info("configuring observability exporter")
	oeConfig := cfg.ObservabilityExporterConfig()
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

	// Setup rate limiter
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &cfg.RateLimit)
	if err != nil {
		return fmt.Errorf("failed to create limiter: %w", err)
	}
	defer limiterStore.Close(ctx)

	smsSigner, err := keys.KeyManagerFor(ctx, &cfg.SMSSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to create sms key manager: %w", err)
	}

	// Setup routes
	mux, err := routes.ENXRedirect(ctx, cfg, db, cacher, smsSigner, limiterStore)
	if err != nil {
		return fmt.Errorf("failed to setup routes: %w", err)
	}

	// Also log requests in local dev.
	if cfg.DevMode {
		mux = handlers.LoggingHandler(os.Stdout, mux)
	}

	// Run server
	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, mux)
}
