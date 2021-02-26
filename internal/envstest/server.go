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
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/assets"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
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
	BadDatabase  *database.Database
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
	Renderer     *render.Renderer
	Server       *server.Server
}

// ProvisionAndLogin provisions and authenticates the initial user, realm,
// permissions on the realm, and session cookie.
func (r *TestServerResponse) ProvisionAndLogin() (*database.Realm, *database.User, *sessions.Session, error) {
	// The migrations scripts create an initial realm and user, fetch them.
	realm, err := r.Database.FindRealm(1)
	if err != nil {
		return nil, nil, nil, err
	}
	user, err := r.Database.FindUser(1)
	if err != nil {
		return nil, nil, nil, err
	}

	// Grant user permission on the realm.
	if err := user.AddToRealm(r.Database, realm, rbac.LegacyRealmAdmin, database.SystemTest); err != nil {
		return nil, nil, nil, err
	}

	// Build the session.
	session, err := r.LoggedInSession(nil, user.Email)
	if err != nil {
		return nil, nil, nil, err
	}

	// Save the current realm on the session.
	controller.StoreSessionRealm(session, realm)

	return realm, user, session, nil
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

// LoggedInSession returns an session with the provided email address logged in.
// It also stores that email verification and MFA prompting have already
// occurred for a consistent post-login experience.
//
// The provided email is marked as verified, has MFA enabled, and is not
// revoked. To test other journeys, manually build the session.
func (r *TestServerResponse) LoggedInSession(session *sessions.Session, email string) (*sessions.Session, error) {
	if session == nil {
		session = &sessions.Session{
			Values:  map[interface{}]interface{}{},
			Options: &sessions.Options{},
			IsNew:   true,
		}
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
		TTL: 30 * time.Minute,
	}); err != nil {
		return nil, err
	}

	return session, nil
}

// NewServer creates a new test UI server instance. See NewHarnessServer for
// more information.
func NewServer(tb testing.TB, testDatabaseInstance *database.TestInstance) *TestServerResponse {
	tb.Helper()
	return NewServerConfig(tb, testDatabaseInstance).NewServer(tb)
}

// ServerConfigResponse is the response from creating a server config.
type ServerConfigResponse struct {
	AuthProvider auth.Provider
	Config       *config.ServerConfig
	Database     *database.Database
	BadDatabase  *database.Database
	Cacher       cache.Cacher
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
	Renderer     *render.Renderer
}

// NewServerConfig creates a new server configuration. It creates all the keys,
// databases, and cacher, but does not actually start the server. All cleanup is
// scheduled by t.Cleanup.
func NewServerConfig(tb testing.TB, testDatabaseInstance *database.TestInstance) *ServerConfigResponse {
	tb.Helper()

	ctx := project.TestContext(tb)

	suffix, err := project.RandomHexString(6)
	if err != nil {
		tb.Fatal(err)
	}

	harness := NewTestHarness(tb, testDatabaseInstance)

	authProvider, err := auth.NewLocal(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	signingKeyManager, ok := harness.KeyManager.(keys.SigningKeyManager)
	if !ok {
		tb.Fatal("kms cannot manage signing keys")
	}
	parent, err := signingKeyManager.CreateSigningKey(ctx, fmt.Sprintf("certificate-%s", suffix), "key")
	if err != nil {
		tb.Fatal(err)
	}
	certificateSigningKey, err := signingKeyManager.CreateKeyVersion(ctx, parent)
	if err != nil {
		tb.Fatal(err)
	}

	// Create the config.
	cfg := &config.ServerConfig{
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

		Database:      *harness.DatabaseConfig,
		Observability: *harness.ObservabilityConfig,
		Cache:         *harness.CacheConfig,

		SMSSigning: config.SMSSigningConfig{
			Keys:       *harness.KeyManagerConfig,
			FailClosed: true,
		},

		Features: config.FeatureConfig{
			EnableAuthenticatedSMS: true,
		},

		CookieKeys:  config.Base64ByteSlice{randomBytes(tb, 64), randomBytes(tb, 32)},
		CSRFAuthKey: randomBytes(tb, 32),

		CertificateSigning: config.CertificateSigningConfig{
			Keys:                  *harness.KeyManagerConfig,
			CertificateSigningKey: certificateSigningKey,
			CertificateIssuer:     "test-iss",
			CertificateAudience:   "test-aud",
		},
		RateLimit: *harness.RateLimiterConfig,

		// DevMode has to be enabled for tests. Otherwise the cookies fail.
		DevMode: true,
	}

	// Process the config - this simulates production setups and also ensures we
	// get the defaults for any unset values.
	emptyLookuper := envconfig.MapLookuper(nil)
	if err := config.ProcessWith(context.Background(), cfg, emptyLookuper); err != nil {
		tb.Fatal(err)
	}

	// Create the renderer.
	renderer, err := render.New(ctx, assets.ServerFS(), true)
	if err != nil {
		tb.Fatal(err)
	}

	return &ServerConfigResponse{
		AuthProvider: authProvider,
		Config:       cfg,
		Database:     harness.Database,
		BadDatabase:  harness.BadDatabase,
		Cacher:       harness.Cacher,
		KeyManager:   harness.KeyManager,
		RateLimiter:  harness.RateLimiter,
		Renderer:     renderer,
	}
}

func (r *ServerConfigResponse) NewServer(tb testing.TB) *TestServerResponse {
	tb.Helper()

	ctx := context.Background()
	mux, err := routes.Server(ctx, r.Config, r.Database, r.AuthProvider, r.Cacher, r.KeyManager, r.KeyManager, r.RateLimiter)
	if err != nil {
		tb.Fatal(err)
	}

	srv := NewHarnessServer(tb, mux)

	return &TestServerResponse{
		AuthProvider: r.AuthProvider,
		Config:       r.Config,
		Database:     r.Database,
		BadDatabase:  r.BadDatabase,
		Cacher:       r.Cacher,
		KeyManager:   r.KeyManager,
		RateLimiter:  r.RateLimiter,
		Renderer:     r.Renderer,
		Server:       srv,
	}
}

// AutoConfirmDialogs automatically clicks "confirm" on popup dialogs from
// window.Confirm prompts.
func AutoConfirmDialogs(ctx context.Context, b bool) <-chan error {
	errCh := make(chan error, 1)

	chromedp.ListenTarget(ctx, func(i interface{}) {
		if _, ok := i.(*page.EventJavascriptDialogOpening); ok {
			go func() {
				select {
				case errCh <- chromedp.Run(ctx, page.HandleJavaScriptDialog(b)):
				default:
				}
			}()
		}
	})

	return errCh
}

// CaptureJavascriptErrors captures any console errors that occur.
func CaptureJavascriptErrors(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)

	chromedp.ListenTarget(ctx, func(i interface{}) {
		go func() {
			if t, ok := i.(*runtime.EventExceptionThrown); ok {
				select {
				case errCh <- fmt.Errorf(t.ExceptionDetails.Error()):
				default:
				}
			}
		}()
	})

	return errCh
}
