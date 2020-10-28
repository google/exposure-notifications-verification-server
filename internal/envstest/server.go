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
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
)

const (
	// sessionName is the name of the session. This must match the session name in
	// the sessions middleware, but cannot be pulled from there due to a cyclical
	// dependency.
	sessionName = "verification-server-session"
)

// TestServerResponse is used as the reply to creating a test UI server.
type TestServerResponse struct {
	AuthProvider auth.Provider
	Cacher       cache.Cacher
	Config       *config.ServerConfig
	Database     *database.Database
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
	Server       *server.Server
}

// SessionCookie returns an encrypted cookie for the given session information,
// capable of being injected into the browser instance and read by the
// application. Since the cookie contains the session, it can be used to mutate
// any server state, including the currently-authenticated user.
func (r *TestServerResponse) SessionCookie(session *sessions.Session) (*http.Cookie, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}

	// Update options to be the server domain
	if session.Options == nil {
		session.Options = &sessions.Options{}
	}
	session.Options.Domain = r.Server.Addr()
	session.Options.Path = "/"

	// Encode and encrypt the cookie using the same configuration as the server.
	codecs := securecookie.CodecsFromPairs(r.Config.CookieKeys.AsBytes()...)
	encoded, err := securecookie.EncodeMulti(sessionName, session.Values, codecs...)
	if err != nil {
		return nil, fmt.Errorf("failed to encode session cookie: %w", err)
	}

	return sessions.NewCookie(sessionName, encoded, session.Options), nil
}

// LoggedInCookie returns an encrypted cookie with the provided email address
// logged in. It also stores that email verification and MFA prompting have
// already occurred for a consistent post-login experience.
//
// The provided email is marked as verified, has MFA enabled, and is not
// revoked. To test other journeys, manually build the session.
func (r *TestServerResponse) LoggedInCookie(email string) (*http.Cookie, error) {
	session := &sessions.Session{
		Values:  map[interface{}]interface{}{},
		Options: &sessions.Options{},
		IsNew:   true,
	}

	controller.StoreSessionEmailVerificationPrompted(session, true)
	controller.StoreSessionMFAPrompted(session, false)

	ctx := context.Background()
	if err := r.AuthProvider.StoreSession(ctx, session, &auth.SessionInfo{
		Data: map[string]interface{}{
			"email":          email,
			"email_verified": true,
			"mfa_enabled":    true,
			"revoked":        false,
		},
		TTL: 5 * time.Minute,
	}); err != nil {
		return nil, err
	}

	return r.SessionCookie(session)
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
	response := newServerConfig(tb)

	// Configure logging
	logger := logging.NewLogger(true)
	ctx := logging.WithLogger(context.Background(), logger)

	// Build the routing.
	mux, err := routes.Server(ctx, response.Config, response.Database, response.AuthProvider, response.Cacher, response.KeyManager, response.RateLimiter)
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
		AuthProvider: response.AuthProvider,
		Config:       response.Config,
		Database:     response.Database,
		Cacher:       response.Cacher,
		KeyManager:   response.KeyManager,
		RateLimiter:  response.RateLimiter,
		Server:       srv,
	}
}

// serverConfigResponse is the response from creating a server config.
type serverConfigResponse struct {
	AuthProvider auth.Provider
	Config       *config.ServerConfig
	Database     *database.Database
	Cacher       cache.Cacher
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
}

// newServerConfig creates a new server configuration. It creates all the keys,
// databases, and cacher, but does not actually start the server. All cleanup is
// scheduled by t.Cleanup.
func newServerConfig(tb testing.TB) *serverConfigResponse {
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
		CookieKeys:  config.Base64ByteSlice{RandomBytes(tb, 64), RandomBytes(tb, 32)},
		CSRFAuthKey: RandomBytes(tb, 32),
		CertificateSigning: config.CertificateSigningConfig{
			// TODO(sethvargo): configure this when the first test requires it
			CertificateSigningKey: "UPDATE_ME",
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
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(context.Background(), cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	return &serverConfigResponse{
		AuthProvider: authProvider,
		Config:       cfg,
		Database:     db,
		Cacher:       cacher,
		KeyManager:   keyManager,
		RateLimiter:  limiterStore,
	}
}

// ServerAssetsPath returns the path to the UI server assets.
func ServerAssetsPath(tb testing.TB) string {
	tb.Helper()
	return filepath.Join(project.Root(), "cmd", "server", "assets")
}
