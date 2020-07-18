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

// This server implements the database cleanup. The server itself is unauthenticated
// and should not be deployed as a public service.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/cleanup"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/sethvargo/go-signalcontext"

	"github.com/google/exposure-notifications-server/pkg/cache"

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

	config, err := config.NewCleanupConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	// Setup database
	db, err := config.Database.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create the router
	r := mux.NewRouter()

	// Cleanup handler doesn't require authentication - does use locking to ensure
	// database isn't tipped over by cleanup.
	cleanupCache, err := cache.New(time.Minute)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	r.Handle("/", cleanup.New(ctx, config, cleanupCache, db)).Methods("GET")

	srv := &http.Server{
		Handler: handlers.CombinedLoggingHandler(os.Stdout, r),
		Addr:    "0.0.0.0:" + strconv.Itoa(config.Port),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Infow("server listening", "port", config.Port)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown server")
	}

	return nil
}
