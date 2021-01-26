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

package routes

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/stats"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit/limitware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/sethvargo/go-limiter"

	"github.com/gorilla/mux"
)

// AdminAPI defines routes for the adminapi service.
func AdminAPI(
	ctx context.Context,
	cfg *config.AdminAPIServerConfig,
	db *database.Database,
	cacher cache.Cacher,
	smsSigner keys.KeyManager,
	limiterStore limiter.Store,
) (http.Handler, error) {
	// Create the router
	r := mux.NewRouter()

	// Common observability context
	ctx, obs := middleware.WithObservability(ctx)
	r.Use(obs)

	// Create the renderer
	h, err := render.New(ctx, "", cfg.DevMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Request ID injection
	populateRequestID := middleware.PopulateRequestID(h)
	r.Use(populateRequestID)

	// Logger injection.
	populateLogger := middleware.PopulateLogger(logging.FromContext(ctx))
	r.Use(populateLogger)

	httplimiter, err := limitware.NewMiddleware(ctx, limiterStore,
		limitware.APIKeyFunc(ctx, db, "adminapi:ratelimit:", cfg.RateLimit.HMACKey),
		limitware.AllowOnError(false))
	if err != nil {
		return nil, fmt.Errorf("failed to create limiter middleware: %w", err)
	}
	rateLimit := httplimiter.Handle

	// Install common security headers
	r.Use(middleware.SecureHeaders(cfg.DevMode, "json"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug()
	r.Use(processDebug)

	// Install the rate limiting first. In this case, we want to limit by key
	// first to reduce the chance of a database lookup.
	r.Use(rateLimit)

	// Other common middlewares
	requireAdminAPIKey := middleware.RequireAPIKey(cacher, db, h, []database.APIKeyType{
		database.APIKeyTypeAdmin,
	})
	requireStatsAPIKey := middleware.RequireAPIKey(cacher, db, h, []database.APIKeyType{
		database.APIKeyTypeStats,
	})
	processFirewall := middleware.ProcessFirewall(h, "adminapi")

	// Health route
	r.Handle("/health", controller.HandleHealthz(db, h)).Methods("GET")

	// API routes
	{
		sub := r.PathPrefix("/api").Subrouter()
		sub.Use(requireAdminAPIKey)
		sub.Use(processFirewall)

		issueapiController := issueapi.New(cfg, db, limiterStore, smsSigner, h)
		sub.Handle("/issue", issueapiController.HandleIssueAPI()).Methods("POST")
		sub.Handle("/batch-issue", issueapiController.HandleBatchIssueAPI()).Methods("POST")

		codesController := codes.NewAPI(ctx, cfg, db, h)
		sub.Handle("/checkcodestatus", codesController.HandleCheckCodeStatus()).Methods("POST")
		sub.Handle("/expirecode", codesController.HandleExpireAPI()).Methods("POST")
	}

	// Stats routes
	{
		sub := r.PathPrefix("/api/stats").Subrouter()
		sub.Use(requireStatsAPIKey)
		sub.Use(processFirewall)

		statsController := stats.New(cacher, db, h)
		sub.Handle("/realm.csv", statsController.HandleRealmStats(stats.StatsTypeCSV)).Methods("GET")
		sub.Handle("/realm.json", statsController.HandleRealmStats(stats.StatsTypeJSON)).Methods("GET")

		sub.Handle("/realm/users.csv", statsController.HandleRealmUsersStats(stats.StatsTypeCSV)).Methods("GET")
		sub.Handle("/realm/users.json", statsController.HandleRealmUsersStats(stats.StatsTypeJSON)).Methods("GET")

		sub.Handle("/realm/users/{id}.csv", statsController.HandleRealmUserStats(stats.StatsTypeCSV)).Methods("GET")
		sub.Handle("/realm/users/{id}.json", statsController.HandleRealmUserStats(stats.StatsTypeJSON)).Methods("GET")

		sub.Handle("/realm/external-issuers.csv", statsController.HandleRealmExternalIssuersStats(stats.StatsTypeCSV)).Methods("GET")
		sub.Handle("/realm/external-issuers.json", statsController.HandleRealmExternalIssuersStats(stats.StatsTypeJSON)).Methods("GET")

		sub.Handle("/realm/key-server.csv", statsController.HandleKeyServerStats(stats.StatsTypeCSV)).Methods("GET")
		sub.Handle("/realm/key-server.json", statsController.HandleKeyServerStats(stats.StatsTypeJSON)).Methods("GET")
	}

	// Wrap the main router in the mutating middleware method. This cannot be
	// inserted as middleware because gorilla processes the method before
	// middleware.
	mux := http.NewServeMux()
	mux.Handle("/", middleware.MutateMethod()(r))
	return mux, nil
}
