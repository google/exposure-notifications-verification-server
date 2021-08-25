// Copyright 2021 the Exposure Notifications Verification Server authors
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

// This server implements scheduled key-rotation. The server itself is unauthenticated
// and should not be deployed as a public service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/rotation"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-server/pkg/server"

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

	cfg, err := config.NewRotationConfig(ctx)
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
	ctx, obs := middleware.WithObservability(ctx)
	logger.Infow("observability exporter", "config", oeConfig)

	// Setup database
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

	// Get token key manager.
	tokenSigner, err := keys.KeyManagerFor(ctx, &cfg.TokenSigning.Keys)
	if err != nil {
		return fmt.Errorf("failed to get token signing key manager: %w", err)
	}
	tokenSignerTyp, ok := tokenSigner.(keys.SigningKeyManager)
	if !ok {
		return fmt.Errorf("token signing key manager is not a signing key manager (is %T)", tokenSigner)
	}

	// Get secret manager.
	secretManager, err := secrets.SecretManagerFor(ctx, &cfg.Secrets)
	if err != nil {
		return fmt.Errorf("failed to get secret manager: %w", err)
	}
	secretManagerTyp, ok := secretManager.(secrets.SecretVersionManager)
	if !ok {
		return fmt.Errorf("secret manager is not a secret version manager (is %T)", secretManager)
	}

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

	rotationController := rotation.New(cfg, db, tokenSignerTyp, secretManagerTyp, h)
	r.Handle("/token-signing-key", rotationController.HandleRotateTokenSigningKey()).Methods(http.MethodGet)
	r.Handle("/realm-verification-keys", rotationController.HandleRotateVerificationKeys()).Methods(http.MethodGet)
	r.Handle("/secrets", rotationController.HandleRotateSecrets()).Methods(http.MethodGet)

	srv, err := server.New(cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", cfg.Port)
	return srv.ServeHTTPHandler(ctx, r)
}
