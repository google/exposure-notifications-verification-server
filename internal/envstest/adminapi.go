// Copyright 2021 Google LLC
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

package envstest

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
)

// AdminAPIServerResponse is the response from a test AdminAPI instance.
type AdminAPIServerResponse struct {
	Config      *config.AdminAPIServerConfig
	Database    *database.Database
	Cacher      cache.Cacher
	KeyManager  keys.KeyManager
	RateLimiter limiter.Store
	Server      *server.Server
}

// NewAdminAPIServer creates a new test AdminAPI server instance. See
// NewHarnessServer for more information.
func NewAdminAPIServer(tb testing.TB, testDatabaseInstance *database.TestInstance) *AdminAPIServerResponse {
	return NewAdminAPIServerConfig(tb, testDatabaseInstance).NewServer(tb)
}

// AdminAPIServerConfigResponse is the response from creating an AdminAPI server
// config.
type AdminAPIServerConfigResponse struct {
	Config      *config.AdminAPIServerConfig
	Database    *database.Database
	Cacher      cache.Cacher
	KeyManager  keys.KeyManager
	RateLimiter limiter.Store
}

// NewAdminAPIServerConfig creates a new API server configuration.
func NewAdminAPIServerConfig(tb testing.TB, testDatabaseInstance *database.TestInstance) *AdminAPIServerConfigResponse {
	tb.Helper()

	harness := NewTestHarness(tb, testDatabaseInstance)

	// Create the config.
	cfg := &config.AdminAPIServerConfig{
		Database:      *harness.DatabaseConfig,
		Observability: *harness.ObservabilityConfig,
		Cache:         *harness.CacheConfig,
		RateLimit:     *harness.RateLimiterConfig,
		SMSSigning: config.SMSSigningConfig{
			Keys:       *harness.KeyManagerConfig,
			FailClosed: true,
		},

		APIKeyCacheDuration:     5 * time.Second,
		ENExpressRedirectDomain: "enx-redirect.local",
		DevMode:                 true,
	}

	// Process the config - this simulates production setups and also ensures we
	// get the defaults for any unset values.
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(context.Background(), cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	return &AdminAPIServerConfigResponse{
		Config:      cfg,
		Database:    harness.Database,
		Cacher:      harness.Cacher,
		KeyManager:  harness.KeyManager,
		RateLimiter: harness.RateLimiter,
	}
}

// NewServer creates a new server.
func (r *AdminAPIServerConfigResponse) NewServer(tb testing.TB) *AdminAPIServerResponse {
	ctx := context.Background()
	mux, err := routes.AdminAPI(ctx, r.Config, r.Database, r.Cacher, r.KeyManager, r.RateLimiter)
	if err != nil {
		tb.Fatal(err)
	}

	srv := NewHarnessServer(tb, mux)

	return &AdminAPIServerResponse{
		Config:      r.Config,
		Database:    r.Database,
		Cacher:      r.Cacher,
		KeyManager:  r.KeyManager,
		RateLimiter: r.RateLimiter,
		Server:      srv,
	}
}
