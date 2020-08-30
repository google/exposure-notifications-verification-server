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
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"

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
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func randomString() string {
	rand.Seed(time.Now().Unix())
	return fmt.Sprintf("%x", rand.Int63())
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	// load configs
	e2eConfig, err := config.NewE2ERunnerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to process e2e-runner config: %w", err)
	}

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
		if err := db.SaveRealm(realm); err != nil {
			return fmt.Errorf("failed to create realm %+v: %w: %v", realm, err, realm.ErrorMessages())
		}
	}

	// Create new API keys
	suffix := randomString()

	adminKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       adminKeyName + suffix,
		APIKeyType: database.APIUserTypeAdmin,
	})
	if err != nil {
		return fmt.Errorf("error trying to create a new Admin API Key: %w", err)
	}

	defer func() {
		app, err := db.FindAuthorizedAppByAPIKey(adminKey)
		if err != nil {
			logger.Errorf("admin API key cleanup failed: %w", err)
		}
		if err := app.Disable(db); err != nil {
			logger.Errorf("admin API key disable failed: %w", err)
		}
		logger.Info("successfully cleaned up e2e test admin key")
	}()

	deviceKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       deviceKeyName + suffix,
		APIKeyType: database.APIUserTypeDevice,
	})
	if err != nil {
		return fmt.Errorf("error trying to create a new Device API Key: %w", err)
	}

	defer func() {
		app, err := db.FindAuthorizedAppByAPIKey(deviceKey)
		if err != nil {
			logger.Errorf("device API key cleanup failed: %w", err)
		}
		if err := app.Disable(db); err != nil {
			logger.Errorf("device API key disable failed: %w", err)
		}
		logger.Info("successfully cleaned up e2e test device key")
	}()

	e2eConfig.VerificationAdminAPIKey = adminKey
	e2eConfig.VerificationAPIServerKey = deviceKey

	// Create the router
	r := mux.NewRouter()
	r.HandleFunc("/default", defaultHandler(ctx, *e2eConfig))
	r.HandleFunc("/revise", reviseHandler(ctx, *e2eConfig))

	srv, err := server.New(e2eConfig.Port)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logger.Infow("server listening", "port", e2eConfig.Port)
	return srv.ServeHTTPHandler(ctx, handlers.CombinedLoggingHandler(os.Stdout, r))
}

func defaultHandler(ctx context.Context, c config.E2ERunnerConfig) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c.DoRevise = false
		if err := e2e(ctx, c); err != nil {
			http.Error(w, "failed (check server logs for more details): "+err.Error(), http.StatusInternalServerError)
		} else {
			fmt.Fprint(w, "ok")
		}
	}
}

func reviseHandler(ctx context.Context, c config.E2ERunnerConfig) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c.DoRevise = true
		if err := e2e(ctx, c); err != nil {
			http.Error(w, "failed (check server logs for more details): "+err.Error(), http.StatusInternalServerError)
		} else {
			fmt.Fprint(w, "ok")
		}
	}
}
