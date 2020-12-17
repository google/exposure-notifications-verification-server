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
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/envstest/testconfig"
	"github.com/google/exposure-notifications-verification-server/internal/routes"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
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
	KeyManager   keys.KeyManager
	RateLimiter  limiter.Store
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

// NewServer creates a new test UI server instance. When this function returns,
// a full UI server will be running locally on a random port. Cleanup is handled
// automatically.
func NewServer(tb testing.TB, testDatabaseInstance *database.TestInstance) *TestServerResponse {
	tb.Helper()

	if testing.Short() {
		tb.Skip()
	}

	// Create the config and requirements.
	response := testconfig.NewServerConfig(tb, testDatabaseInstance)

	ctx := context.Background()

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
