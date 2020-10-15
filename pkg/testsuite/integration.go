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

package testsuite

import (
	"context"
	"crypto/sha1"
	"os"
	"testing"

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

// IntegrationSuite contains the integration test configs and other useful data.
type IntegrationSuite struct {
	cfg *config.IntegrationTestConfig

	db    *database.Database
	realm *database.Realm

	adminKey, deviceKey string

	adminSrv *server.Server
	apiSrv   *server.Server
}

// NewIntegrationSuite creates a IntegrationSuite for integration tests.
func NewIntegrationSuite(tb testing.TB, ctx context.Context) *IntegrationSuite {
	tb.Helper()
	cfg, db := config.NewIntegrationTestConfig(ctx, tb)
	if err := db.Open(ctx); err != nil {
		tb.Fatalf("failed to connect to database: %v", err)
	}
	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Errorf("failed to close db: %v", err)
		}
	})
	// Create or reuse the existing realm
	realm, err := db.FindRealmByName(realmName)
	if err != nil {
		if !database.IsNotFound(err) {
			tb.Fatalf("error when finding the realm %q: %v", realmName, err)
		}
		realm = database.NewRealmWithDefaults(realmName)
		realm.RegionCode = realmRegionCode
		if err := db.SaveRealm(realm, database.System); err != nil {
			tb.Fatalf("failed to create realm %+v: %v: %v", realm, err, realm.ErrorMessages())
		}
	}

	// Create new API keys
	suffix, err := randomString()
	if err != nil {
		tb.Fatalf("failed to create suffix string for API keys: %v", err)
	}

	adminKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       adminKeyName + suffix,
		APIKeyType: database.APIKeyTypeAdmin,
	}, database.System)
	if err != nil {
		tb.Fatalf("error trying to create a new Admin API Key: %v", err)
	}

	deviceKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       deviceKeyName + suffix,
		APIKeyType: database.APIKeyTypeDevice,
	}, database.System)
	if err != nil {
		tb.Fatalf("error trying to create a new Device API Key: %v", err)
	}

	return &IntegrationSuite{
		cfg:       cfg,
		db:        db,
		realm:     realm,
		adminKey:  adminKey,
		deviceKey: deviceKey,
	}
}

// NewAdminAPIClient runs an Admin API Server and returns a corresponding client.
func (s *IntegrationSuite) NewAdminAPIClient(ctx context.Context, tb testing.TB) (*AdminClient, error) {
	srv := s.newAdminAPIServer(ctx, tb)
	s.adminSrv = srv
	addr := "http://[::1]:" + srv.Port()
	return NewAdminClient(addr, s.adminKey)
}

// NewAPIClient runs an API Server and returns a corresponding client.
func (s *IntegrationSuite) NewAPIClient(ctx context.Context, tb testing.TB) (*APIClient, error) {
	srv := s.newAPIServer(ctx, tb)
	s.apiSrv = srv
	addr := "http://[::1]:" + srv.Port()
	return NewAPIClient(addr, s.deviceKey)
}

func (s *IntegrationSuite) newAdminAPIServer(ctx context.Context, tb testing.TB) *server.Server {
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
		tb.Fatalf("failed to create the renderer %v", err)
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
			tb.Fatalf("failed to close cacher: %v", err)
		}
	})

	// Create LimitStore
	limiterStore, err := ratelimit.RateLimiterFor(ctx, &s.cfg.AdminAPISrvConfig.RateLimit)
	if err != nil {
		tb.Fatalf("failed to create the limit store %v", err)
	}

	adminRouter.Handle("/health", controller.HandleHealthz(ctx, &s.cfg.AdminAPISrvConfig.Database, h)).Methods("GET")

	{
		sub := adminRouter.PathPrefix("/api").Subrouter()

		// Setup API auth
		requireAPIKey := middleware.RequireAPIKey(ctx, cacher, s.db, h, []database.APIKeyType{
			database.APIKeyTypeAdmin,
		})
		// Install the APIKey Auth Middleware
		sub.Use(requireAPIKey)

		issueapiController, err := issueapi.New(ctx, &s.cfg.AdminAPISrvConfig, s.db, limiterStore, h)
		if err != nil {
			tb.Fatalf("failed to create issue api controller: %v", err)
		}
		sub.Handle("/issue", issueapiController.HandleIssue()).Methods("POST")

		codeStatusController := codestatus.NewAPI(ctx, &s.cfg.AdminAPISrvConfig, s.db, h)
		sub.Handle("/checkcodestatus", codeStatusController.HandleCheckCodeStatus()).Methods("POST")
		sub.Handle("/expirecode", codeStatusController.HandleExpireAPI()).Methods("POST")
	}

	srv, err := server.New(s.cfg.AdminAPISrvConfig.Port)
	if err != nil {
		tb.Fatalf("failed to create server: %v", err)
	}

	// Stop the server on cleanup
	stopCtx, stop := context.WithCancel(ctx)
	tb.Cleanup(stop)

	go func() {
		if err := srv.ServeHTTPHandler(stopCtx, handlers.CombinedLoggingHandler(os.Stdout, adminRouter)); err != nil {
			tb.Fatalf("failed to serve HTTP handler: %v", err)
		}
	}()
	return srv
}

func (s *IntegrationSuite) newAPIServer(ctx context.Context, tb testing.TB) *server.Server {
	// Create the renderer
	h, err := render.New(ctx, "", s.cfg.APISrvConfig.DevMode)
	if err != nil {
		tb.Fatalf("failed to create the renderer %v", err)
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
			tb.Fatalf("failed to close cacher: %v", err)
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
		requireAPIKey := middleware.RequireAPIKey(ctx, cacher, s.db, h, []database.APIKeyType{
			database.APIKeyTypeDevice,
		})
		// Install the APIKey Auth Middleware
		sub.Use(requireAPIKey)

		verifyChaff := chaff.New()
		defer verifyChaff.Close()
		verifyapiController, err := verifyapi.New(ctx, &s.cfg.APISrvConfig, s.db, h, tokenSigner)
		if err != nil {
			tb.Fatalf("failed to create verify api controller: %v", err)
		}
		sub.Handle("/verify", verifyapiController.HandleVerify()).Methods("POST")

		certChaff := chaff.New()
		defer certChaff.Close()
		certapiController, err := certapi.New(ctx, &s.cfg.APISrvConfig, s.db, cacher, certificateSigner, h)
		if err != nil {
			tb.Fatalf("failed to create cert api controller: %v", err)
		}
		sub.Handle("/certificate", certapiController.HandleCertificate()).Methods("POST")
	}

	srv, err := server.New(s.cfg.APISrvConfig.Port)
	if err != nil {
		tb.Fatalf("failed to create server: %v", err)
	}

	// Stop the server on cleanup
	stopCtx, stop := context.WithCancel(ctx)
	tb.Cleanup(stop)

	go func() {
		if err := srv.ServeHTTPHandler(stopCtx, handlers.CombinedLoggingHandler(os.Stdout, apiRouter)); err != nil {
			tb.Fatalf("failed to serve HTTP handler: %v", err)
		}
	}()
	return srv
}
