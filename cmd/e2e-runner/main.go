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

// This server is a simple webserver that triggers the e2e-test binary.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sethvargo/go-signalcontext"
)

const (
	realmName       = "e2e-test-realm"
	realmRegionCode = "e2e-test"
	adminKeyName    = "e2e-admin-key."
	deviceKeyName   = "e2e-device-key."
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

// Generate random string of 32 characters in length
func randomString() (string, error) {
	b := make([]byte, 512)
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to generate random: %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b[:])), nil
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
	if err := oe.StartExporter(); err != nil {
		return fmt.Errorf("error initializing observability exporter: %w", err)
	}
	defer oe.Close()
	logger.Infow("observability exporter", "config", e2eConfig.Observability)

	db, err := e2eConfig.Database.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Create or reuse the existing realm
	realm, err := db.FindRealmByName(realmName)
	if err != nil {
		if !database.IsNotFound(err) {
			return fmt.Errorf("error when finding the realm %q: %w", realmName, err)
		}
		realm = database.NewRealmWithDefaults(realmName)
		realm.RegionCode = realmRegionCode
		if err := db.SaveRealm(realm, database.System); err != nil {
			return fmt.Errorf("failed to create realm %+v: %w: %v", realm, err, realm.ErrorMessages())
		}
	}

	// Create new API keys
	suffix, err := randomString()
	if err != nil {
		return fmt.Errorf("failed to create suffix string for API keys: %w", err)
	}

	adminKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       adminKeyName + suffix,
		APIKeyType: database.APIKeyTypeAdmin,
	}, database.System)
	if err != nil {
		return fmt.Errorf("error trying to create a new Admin API Key: %w", err)
	}

	defer func() {
		app, err := db.FindAuthorizedAppByAPIKey(adminKey)
		if err != nil {
			logger.Errorf("admin API key cleanup failed: %w", err)
		}
		now := time.Now().UTC()
		app.DeletedAt = &now
		if err := db.SaveAuthorizedApp(app, database.System); err != nil {
			logger.Errorf("admin API key disable failed: %w", err)
		}
		logger.Info("successfully cleaned up e2e test admin key")
	}()

	deviceKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       deviceKeyName + suffix,
		APIKeyType: database.APIKeyTypeDevice,
	}, database.System)
	if err != nil {
		return fmt.Errorf("error trying to create a new Device API Key: %w", err)
	}

	defer func() {
		app, err := db.FindAuthorizedAppByAPIKey(deviceKey)
		if err != nil {
			logger.Errorf("device API key cleanup failed: %w", err)
			return
		}
		now := time.Now().UTC()
		app.DeletedAt = &now
		if err := db.SaveAuthorizedApp(app, database.System); err != nil {
			logger.Errorf("device API key disable failed: %w", err)
		}
		logger.Info("successfully cleaned up e2e test device key")
	}()

	e2eConfig.TestConfig.VerificationAdminAPIKey = adminKey
	e2eConfig.TestConfig.VerificationAPIServerKey = deviceKey

	// Create the router
	r := mux.NewRouter()
	r.HandleFunc("/default", defaultHandler(ctx, e2eConfig.TestConfig))
	r.HandleFunc("/revise", reviseHandler(ctx, e2eConfig.TestConfig))

	srv, err := server.New(e2eConfig.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", e2eConfig.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

// Config is passed by value so that each http hadndler has a separate copy (since they are changing one of the)
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
