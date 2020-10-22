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

package envstest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
)

// TestServerResponse is used as the reply to creating a test UI server.
type TestServerResponse struct {
	Config      *config.ServerConfig
	Database    *database.Database
	Cacher      cache.Cacher
	KeyManager  keys.KeyManager
	RateLimiter limiter.Store
	Server      *server.Server
}

// NewServer creates a new test UI server instance. When this function returns,
// a full UI server will be running locally on a random port. Cleanup is handled
// automatically.
func NewServer(tb testing.TB) *TestServerResponse {
	tb.Helper()

	if testing.Short() {
		tb.Skip()
	}

	// Create the config and requirements.
	response := NewServerConfig(tb)

	// Build the routing.
	ctx := context.Background()
	mux, err := routes.Server(ctx, response.Config, response.Database, response.Cacher, response.KeyManager, response.RateLimiter)
	if err != nil {
		tb.Fatal(err)
	}

	// Create a stoppable context.
	doneCtx, cancel := context.WithCancel(ctx)
	tb.Cleanup(func() {
		cancel()
	})

	// Start the server on a random port. Closing doneCtx will stop the server
	// (which the cleanup step does).
	srv, err := server.New("")
	if err != nil {
		tb.Fatal(err)
	}
	go func() {
		if err := srv.ServeHTTPHandler(doneCtx, mux); err != nil {
			tb.Error(err)
		}
	}()

	return &TestServerResponse{
		Config:      response.Config,
		Database:    response.Database,
		Cacher:      response.Cacher,
		KeyManager:  response.KeyManager,
		RateLimiter: response.RateLimiter,
		Server:      srv,
	}
}

// ServerConfigResponse is the response from creating a server config.
type ServerConfigResponse struct {
	Config      *config.ServerConfig
	Database    *database.Database
	Cacher      cache.Cacher
	KeyManager  keys.KeyManager
	RateLimiter limiter.Store
}

// NewServerConfig creates a new server configuration. It creates all the keys,
// databases, and cacher, but does not actually start the server. All cleanup is
// scheduled by t.Cleanup.
func NewServerConfig(tb testing.TB) *ServerConfigResponse {
	tb.Helper()

	if testing.Short() {
		tb.Skip()
	}

	// Create the cacher.
	cacher, err := cache.NewInMemory(nil)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := cacher.Close(); err != nil {
			tb.Fatal(err)
		}
	})

	// Create the database.
	db, dbConfig := database.NewTestDatabaseWithCacher(tb, cacher)

	// Create the key manager.
	keyManager := keys.TestKeyManager(tb)

	// Create the rate limiter.
	limiterStore, err := memorystore.New(&memorystore.Config{
		Tokens:   30,
		Interval: time.Second,
	})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := limiterStore.Close(ctx); err != nil {
			tb.Fatal(err)
		}
	})

	// Create the config.
	cfg := &config.ServerConfig{
		AssetsPath: ServerAssetsPath(tb),
		Cache: cache.Config{
			Type:    cache.TypeInMemory,
			HMACKey: RandomBytes(tb, 64),
		},
		Database: *dbConfig,

		// TODO(sethvargo): source these from the environment. They aren't
		// "secrets", but people should be able to use their own.
		Firebase: config.FirebaseConfig{
			APIKey:          "AIzaSyAm3J2LnU95nl4imVISqZk_zTdRbUzzlow",
			AuthDomain:      "apollo-server-273118.firebaseapp.com",
			DatabaseURL:     "https://apollo-server-273118.firebaseio.com",
			ProjectID:       "apollo-server-273118",
			StorageBucket:   "apollo-server-273118.appspot.com",
			MessageSenderID: "38554818207",
			AppID:           "1:38554818207:web:b55eca99f6d233ed08b4aa",
			MeasurementID:   "G-J04182V10C",
		},

		CookieKeys:  config.Base64ByteSlice{RandomBytes(tb, 64), RandomBytes(tb, 32)},
		CSRFAuthKey: RandomBytes(tb, 32),
		CertificateSigning: config.CertificateSigningConfig{
			CertificateSigningKey: "UPDATE_ME", // TODO(sethvargo): configure this when the first test requires it
			Keys: keys.Config{
				KeyManagerType: keys.KeyManagerTypeFilesystem,
				FilesystemRoot: filepath.Join(project.Root(), "local", "test", RandomString(tb, 8)),
			},
		},
		RateLimit: ratelimit.Config{
			Type:    ratelimit.RateLimiterTypeMemory,
			HMACKey: RandomBytes(tb, 64),
		},

		// DevMode has to be enabled for tests. Otherwise the cookies fail.
		DevMode: true,
	}

	// Process the config - this simulates production setups and also ensures we
	// get the defaults for any unset values.
	ctx := context.Background()
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(ctx, cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	return &ServerConfigResponse{
		Config:      cfg,
		Database:    db,
		Cacher:      cacher,
		KeyManager:  keyManager,
		RateLimiter: limiterStore,
	}
}

// ServerAssetsPath returns the path to the UI server assets.
func ServerAssetsPath(tb testing.TB) string {
	tb.Helper()
	return filepath.Join(project.Root(), "cmd", "server", "assets")
}
