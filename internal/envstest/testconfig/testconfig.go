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

package testconfig

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
)

// ServerConfigResponse is the response from creating a server config.
type ServerConfigResponse struct {
	AuthProvider auth.Provider
	Config       *config.ServerConfig
	Database     *database.Database
	Cacher       cache.Cacher
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
}

// NewServerConfig creates a new server configuration. It creates all the keys,
// databases, and cacher, but does not actually start the server. All cleanup is
// scheduled by t.Cleanup.
func NewServerConfig(tb testing.TB, testDatabaseInstance *database.TestInstance) *ServerConfigResponse {
	tb.Helper()

	if testing.Short() {
		tb.Skip()
	}

	// Create the auth provider
	authProvider, err := auth.NewLocal(context.Background())
	if err != nil {
		tb.Fatal(err)
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
	db, dbConfig := testDatabaseInstance.NewDatabase(tb, cacher)

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
		AssetsPath:  ServerAssetsPath(),
		LocalesPath: LocalesPath(),
		Cache: cache.Config{
			Type:    cache.TypeInMemory,
			HMACKey: randomBytes(tb, 64),
		},
		Database: *dbConfig,
		// Firebase is not used for browser tests.
		Firebase: config.FirebaseConfig{
			APIKey:          "test",
			AuthDomain:      "test.firebaseapp.com",
			DatabaseURL:     "https://test.firebaseio.com",
			ProjectID:       "test",
			StorageBucket:   "test.appspot.com",
			MessageSenderID: "test",
			AppID:           "1:test:web:test",
			MeasurementID:   "G-TEST",
		},
		CookieKeys:  config.Base64ByteSlice{randomBytes(tb, 64), randomBytes(tb, 32)},
		CSRFAuthKey: randomBytes(tb, 32),
		CertificateSigning: config.CertificateSigningConfig{
			CertificateSigningKey: "UPDATE_ME",
			Keys: keys.Config{
				KeyManagerType: keys.KeyManagerTypeFilesystem,
				FilesystemRoot: filepath.Join(project.Root(), "local", "test", randomString(tb, 8)),
			},
		},
		RateLimit: ratelimit.Config{
			Type:    ratelimit.RateLimiterTypeMemory,
			HMACKey: randomBytes(tb, 64),
		},

		// DevMode has to be enabled for tests. Otherwise the cookies fail.
		DevMode: true,
	}

	// Process the config - this simulates production setups and also ensures we
	// get the defaults for any unset values.
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(context.Background(), cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	return &ServerConfigResponse{
		AuthProvider: authProvider,
		Config:       cfg,
		Database:     db,
		Cacher:       cacher,
		KeyManager:   keyManager,
		RateLimiter:  limiterStore,
	}
}

// AutoConfirmDialogs automatically clicks "confirm" on popup dialogs from
// window.Confirm prompts.
func AutoConfirmDialogs(ctx context.Context, b bool) <-chan error {
	errCh := make(chan error, 1)

	chromedp.ListenTarget(ctx, func(i interface{}) {
		if _, ok := i.(*page.EventJavascriptDialogOpening); ok {
			go func() {
				errCh <- chromedp.Run(ctx, page.HandleJavaScriptDialog(b))
			}()
		}
	})

	return errCh
}

// ServerAssetsPath returns the path to the UI server assets.
func ServerAssetsPath() string {
	return filepath.Join(project.Root(), "cmd", "server", "assets")
}

// LocalesPath returns the path to the i18n locales.
func LocalesPath() string {
	return filepath.Join(project.Root(), "internal", "i18n", "locales")
}

func randomBytes(tb testing.TB, len int) []byte {
	tb.Helper()

	b, err := project.RandomBytes(len)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func randomString(tb testing.TB, len int) string {
	tb.Helper()

	s, err := project.RandomHexString(len)
	if err != nil {
		tb.Fatal(err)
	}
	return s
}
