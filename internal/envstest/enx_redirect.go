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
	"path/filepath"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/sethvargo/go-envconfig"
)

// ENXRedirectServerResponse is the response from a test ENX redirect instance.
type ENXRedirectServerResponse struct {
	Config   *config.RedirectConfig
	Database *database.Database
	Cacher   cache.Cacher
	Server   *server.Server
}

// NewENXRedirectServer creates a new test ENX redirect server instance. See
// NewHarnessServer for more information.
func NewENXRedirectServer(tb testing.TB, testDatabaseInstance *database.TestInstance) *ENXRedirectServerResponse {
	return NewENXRedirectServerConfig(tb, testDatabaseInstance).NewServer(tb)
}

// ENXRedirectServerConfigResponse is the response from creating an Redirect server
// config.
type ENXRedirectServerConfigResponse struct {
	Config   *config.RedirectConfig
	Database *database.Database
	Cacher   cache.Cacher
}

// NewENXRedirectServerConfig creates a new ENX redirect server configuration.
func NewENXRedirectServerConfig(tb testing.TB, testDatabaseInstance *database.TestInstance) *ENXRedirectServerConfigResponse {
	tb.Helper()

	harness := NewTestHarness(tb, testDatabaseInstance)

	// Create the config.
	cfg := &config.RedirectConfig{
		Database:      *harness.DatabaseConfig,
		Observability: *harness.ObservabilityConfig,
		Cache:         *harness.CacheConfig,
		AssetsPath:    ENXRedirectAssetsPath(),
		HostnameConfig: map[string]string{
			"e2e-test.test.local": "e2e-test",
		},

		DevMode: true,
	}

	// Process the config - this simulates production setups and also ensures we
	// get the defaults for any unset values.
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(context.Background(), cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	return &ENXRedirectServerConfigResponse{
		Config:   cfg,
		Database: harness.Database,
		Cacher:   harness.Cacher,
	}
}

// NewServer creates a new server.
func (r *ENXRedirectServerConfigResponse) NewServer(tb testing.TB) *ENXRedirectServerResponse {
	ctx := context.Background()
	mux, err := routes.ENXRedirect(ctx, r.Config, r.Database, r.Cacher)
	if err != nil {
		tb.Fatal(err)
	}

	srv := NewHarnessServer(tb, mux)

	return &ENXRedirectServerResponse{
		Config:   r.Config,
		Database: r.Database,
		Cacher:   r.Cacher,
		Server:   srv,
	}
}

// ENXRedirectAssetsPath returns the path to the ENX redirector assets.
func ENXRedirectAssetsPath() string {
	return filepath.Join(project.Root(), "cmd", "enx-redirect", "assets")
}
