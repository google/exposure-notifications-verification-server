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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
)

// NewHarnessServer creates a new server for the mux on a random port. Cleanup
// is handled automatically.
func NewHarnessServer(tb testing.TB, mux http.Handler) *server.Server {
	// Inject the test logger into the context instead of the default sugared
	// logger.
	mux = middleware.PopulateLogger(project.TestLogger(tb))(mux)

	// Create a stoppable context.
	doneCtx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(func() {
		cancel()
	})

	// As of 2020-10-29, our CI infrastructure does not support IPv6. `server.New`
	// binds to "tcp", which picks the "best" address, but it prefers IPv6. As a
	// result, the server binds to the IPv6 loopback`[::]`, but then our browser
	// instance cannot actually contact that loopback interface. To mitigate this,
	// create a custom listener and force IPv4. The listener will still pick a
	// randomly available port, but it will only choose an IPv4 address upon which
	// to bind.
	listener, err := net.Listen("tcp4", ":0")
	if err != nil {
		tb.Fatalf("failed to create listener: %v", err)
	}

	// Start the server on a random port. Closing doneCtx will stop the server
	// (which the cleanup step does).
	srv, err := server.NewFromListener(listener)
	if err != nil {
		tb.Fatal(err)
	}
	go func() {
		if err := srv.ServeHTTPHandler(doneCtx, mux); err != nil {
			tb.Error(err)
		}
	}()

	return srv
}

type TestHarnessResponse struct {
	Cacher      cache.Cacher
	CacheConfig *cache.Config

	Database       *database.Database
	DatabaseConfig *database.Config

	ObservabilityConfig *observability.Config

	KeyManager       keys.KeyManager
	KeyManagerConfig *keys.Config

	RateLimiter       limiter.Store
	RateLimiterConfig *ratelimit.Config
}

func NewTestHarness(tb testing.TB, testDatabaseInstance *database.TestInstance) *TestHarnessResponse {
	tb.Helper()

	if testing.Short() {
		tb.Skip()
	}

	ctx := project.TestContext(tb)

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
	cacheConfig := &cache.Config{
		Type:    cache.TypeInMemory,
		HMACKey: randomBytes(tb, 64),
	}

	// Create the key manager.
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			tb.Fatal(err)
		}
	})
	keyManager, err := keys.NewFilesystem(ctx, &keys.Config{
		FilesystemRoot: tmpdir,
	})
	if err != nil {
		tb.Fatal(err)
	}
	signingKeyManager, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		tb.Fatalf("%T is not SigningKeyManager", keyManager)
	}
	keyManagerConfig := &keys.Config{
		Type:           "FILESYSTEM",
		FilesystemRoot: tmpdir,
	}

	// Create the database.
	db, dbConfig := testDatabaseInstance.NewDatabase(tb, cacher,
		database.WithKeyManager(keyManagerConfig, keyManager),
		database.WithSigningKeyManager(keyManagerConfig, signingKeyManager))

	// Create the rate limiter.
	limiterStore, err := memorystore.New(&memorystore.Config{
		Tokens:   30,
		Interval: time.Second,
	})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := limiterStore.Close(ctx); err != nil {
			tb.Fatal(err)
		}
	})
	limiterConfig := &ratelimit.Config{
		Type:    ratelimit.RateLimiterTypeMemory,
		HMACKey: randomBytes(tb, 64),
	}

	return &TestHarnessResponse{
		Cacher:      cacher,
		CacheConfig: cacheConfig,

		Database:       db,
		DatabaseConfig: dbConfig,

		ObservabilityConfig: &observability.Config{ExporterType: "NOOP"},

		KeyManager:       keyManager,
		KeyManagerConfig: keyManagerConfig,

		RateLimiter:       limiterStore,
		RateLimiterConfig: limiterConfig,
	}
}

func randomBytes(tb testing.TB, length int) []byte {
	tb.Helper()

	b, err := project.RandomBytes(length)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}
