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

package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codestatus"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/verifyapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
)

const (
	realmName       = "test-realm"
	realmRegionCode = "test"
	adminKeyName    = "integration-admin-key"
	deviceKeyName   = "integration-device-key"
)

// Suite contains the integration test configs and other useful data.
type Suite struct {
	cfg *config.IntegrationTestConfig

	db    *database.Database
	realm *database.Realm

	adminKey, deviceKey string

	adminSrv *server.Server
	apiSrv   *server.Server
}

// NewTestSuite creates a Suite for integration tests.
func NewTestSuite(tb testing.TB, ctx context.Context) *Suite {
	tb.Helper()
	cfg, db := config.NewIntegrationTestConfig(ctx, tb)

	// Create or reuse the existing realm
	realm, err := db.FindRealmByName(realmName)
	if err != nil {
		if !database.IsNotFound(err) {
			tb.Errorf("error when finding the realm %q: %w", realmName, err)
		}
		realm = database.NewRealmWithDefaults(realmName)
		realm.RegionCode = realmRegionCode
		if err := db.SaveRealm(realm); err != nil {
			tb.Errorf("failed to create realm %+v: %w: %v", realm, err, realm.ErrorMessages())
		}
	}

	// Create new API keys
	suffix, err := randomString()
	if err != nil {
		tb.Errorf("failed to create suffix string for API keys: %w", err)
	}

	adminKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       adminKeyName + suffix,
		APIKeyType: database.APIUserTypeAdmin,
	})
	if err != nil {
		tb.Errorf("error trying to create a new Admin API Key: %w", err)
	}

	deviceKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       deviceKeyName + suffix,
		APIKeyType: database.APIUserTypeDevice,
	})
	if err != nil {
		tb.Errorf("error trying to create a new Device API Key: %w", err)
	}

	return &Suite{
		cfg:       cfg,
		db:        db,
		realm:     realm,
		adminKey:  adminKey,
		deviceKey: deviceKey,
	}
}

// NewAdminAPIServer runs an Admin API Server and returns a corresponding client.
func (s *Suite) NewAdminAPIServer(ctx context.Context, tb testing.TB) *AdminClient {
	srv := s.newAdminAPIServer(ctx, tb)
	s.adminSrv = srv
	return NewAdminClient(srv.Addr(), s.adminKey)
}

// NewAPIServer runs an API Server and returns a corresponding client.
func (s *Suite) NewAPIServer(ctx context.Context, tb testing.TB) *APIClient {
	srv := s.newAPIServer(ctx, tb)
	s.apiSrv = srv
	return NewAPIClient(srv.Addr(), s.deviceKey)
}

func (s *Suite) newAdminAPIServer(ctx context.Context, tb testing.TB) *server.Server {
	// Create the router
	adminRouter := mux.NewRouter()
	// Install common security headers
	adminRouter.Use(middleware.SecureHeaders(ctx, s.cfg.AdminAPISrvConfig.DevMode, "json"))

	// Enable debug headers
	processDebug := middleware.ProcessDebug(ctx)
	adminRouter.Use(processDebug)

	// Create the renderer
	h, err := render.New(ctx, "", s.cfg.APISrvConfig.DevMode)
	if err != nil {
		tb.Errorf("failed to create the renderer %v", err)
	}

	// Setup cacher
	cacher, err := cache.CacherFor(ctx, &s.cfg.APISrvConfig.Cache, cache.MultiKeyFunc(
		cache.HMACKeyFunc(sha1.New, s.cfg.APISrvConfig.Cache.HMACKey),
		cache.PrefixKeyFunc("apiserver:cache:"),
	))
	if err != nil {
		tb.Fatalf("failed to create cacher: %v", err)
	}
	tb.Cleanup(func() {
		if err := cacher.Close(); err != nil {
			tb.Errorf("got err when cleanup: %v", err)
		}
	})

	// Create LimitStore
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &s.cfg.AdminAPISrvConfig.RateLimit)
	if err != nil {
		tb.Errorf("failed to create the limit store %v", err)
	}

	adminRouter.Handle("/health", controller.HandleHealthz(ctx, &s.cfg.AdminAPISrvConfig.Database, h)).Methods("GET")

	{
		sub := adminRouter.PathPrefix("/api").Subrouter()

		// Setup API auth
		requireAPIKey := middleware.RequireAPIKey(ctx, cacher, s.db, h, []database.APIUserType{
			database.APIUserTypeAdmin,
		})
		// Install the APIKey Auth Middleware
		sub.Use(requireAPIKey)

		issueapiController, err := issueapi.New(ctx, &s.cfg.AdminAPISrvConfig, s.db, limiterStore, h)
		if err != nil {
			tb.Errorf("issueapi.New: %w", err)
		}
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")

		codeStatusController := codestatus.NewAPI(ctx, &s.cfg.AdminAPISrvConfig, s.db, h)
		sub.Handle("/checkcodestatus", codeStatusController.HandleCheckCodeStatus()).Methods("POST")
		sub.Handle("/expirecode", codeStatusController.HandleExpireAPI()).Methods("POST")
	}

	srv, err := server.New(s.cfg.AdminAPISrvConfig.Port)
	if err != nil {
		tb.Errorf("failed to create server: %w", err)
	}

	// Stop the server on cleanup
	stopCtx, stop := context.WithCancel(ctx)
	tb.Cleanup(stop)

	go func() {
		if err := srv.ServeHTTPHandler(stopCtx, handlers.CombinedLoggingHandler(os.Stdout, adminRouter)); err != nil {
			tb.Error(err)
		}
	}()
	return srv
}

func (s *Suite) newAPIServer(ctx context.Context, tb testing.TB) *server.Server {
	// Create the renderer
	h, err := render.New(ctx, "", s.cfg.APISrvConfig.DevMode)
	if err != nil {
		tb.Errorf("failed to create the renderer %v", err)
	}

	// Setup cacher
	cacher, err := cache.CacherFor(ctx, &s.cfg.APISrvConfig.Cache, cache.MultiKeyFunc(
		cache.HMACKeyFunc(sha1.New, s.cfg.APISrvConfig.Cache.HMACKey),
		cache.PrefixKeyFunc("apiserver:cache:"),
	))
	if err != nil {
		tb.Fatalf("failed to create cacher: %v", err)
	}
	tb.Cleanup(func() {
		if err := cacher.Close(); err != nil {
			tb.Errorf("got err when cleanup: %v", err)
		}
	})

	// Setup signers
	tokenSigner, err := keys.KeyManagerFor(ctx, &s.cfg.APISrvConfig.TokenSigning.Keys)
	if err != nil {
		tb.Fatalf("failed to create token key manager: %v", err)
	}
	certificateSigner, err := keys.KeyManagerFor(ctx, &s.cfg.APISrvConfig.CertificateSigning.Keys)
	if err != nil {
		tb.Fatalf("failed to create certificate key manager: %v", err)
	}

	apiRouter := mux.NewRouter()
	// Install common security headers
	apiRouter.Use(middleware.SecureHeaders(ctx, s.cfg.APISrvConfig.DevMode, "json"))

	apiRouter.Handle("/health", controller.HandleHealthz(ctx, &s.cfg.APISrvConfig.Database, h)).Methods("GET")

	{
		sub := apiRouter.PathPrefix("/api").Subrouter()

		// Setup API auth
		requireAPIKey := middleware.RequireAPIKey(ctx, cacher, s.db, h, []database.APIUserType{
			database.APIUserTypeDevice,
		})
		// Install the APIKey Auth Middleware
		sub.Use(requireAPIKey)

		verifyChaff := chaff.New()
		defer verifyChaff.Close()
		verifyapiController, err := verifyapi.New(ctx, &s.cfg.APISrvConfig, s.db, h, tokenSigner)
		if err != nil {
			tb.Errorf("failed to create verify api controller: %w", err)
		}
		sub.Handle("/verify", verifyapiController.HandleVerify()).Methods("POST")

		certChaff := chaff.New()
		defer certChaff.Close()
		certapiController, err := certapi.New(ctx, &s.cfg.APISrvConfig, s.db, cacher, certificateSigner, h)
		if err != nil {
			tb.Errorf("failed to create certapi controller: %w", err)
		}
		sub.Handle("/certificate", certapiController.HandleCertificate()).Methods("POST")
	}

	srv, err := server.New(s.cfg.APISrvConfig.Port)
	if err != nil {
		tb.Errorf("failed to create server: %w", err)
	}

	// Stop the server on cleanup
	stopCtx, stop := context.WithCancel(ctx)
	tb.Cleanup(stop)

	go func() {
		if err := srv.ServeHTTPHandler(stopCtx, handlers.CombinedLoggingHandler(os.Stdout, apiRouter)); err != nil {
			tb.Error(err)
		}
	}()
	return srv
}

type prefixRoundTripper struct {
	addr string
	rt   http.RoundTripper
}

// RoundTrip wraps transport's RoutTrip and sets the scheme and host address.
func (p *prefixRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	u := r.URL
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if u.Host == "" {
		u.Host = p.addr
	}

	return p.rt.RoundTrip(r)
}

// NewAdminClient creates an Admin API test client.
func NewAdminClient(addr, key string) *AdminClient {
	prt := &prefixRoundTripper{
		addr: addr,
		rt:   http.DefaultTransport,
	}
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: prt,
	}
	return &AdminClient{
		client: httpClient,
		key:    key,
	}
}

// NewAPIClient creates an API server test client.
func NewAPIClient(addr, key string) *APIClient {
	prt := &prefixRoundTripper{
		addr: addr,
		rt:   http.DefaultTransport,
	}
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: prt,
	}
	return &APIClient{
		client: httpClient,
		key:    key,
	}
}

func randomString() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04x", n), nil
}
