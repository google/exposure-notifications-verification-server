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
	"testing"

	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig"
)

// E2EConfig represents configurations to run server E2E tests.
type E2EConfig struct {
	APIServerURL string           `env:"E2E_APISERVER_URL"`
	AdminAPIURL  string           `env:"E2E_ADMINAPI_URL"`
	ProjectID    string           `env:"E2E_PROJECT_ID"`
	DBConfig     *database.Config `env:",prefix=E2E_"`
}

// NewE2EConfig returns a new E2E test config.
func NewE2EConfig(tb testing.TB, ctx context.Context) *E2EConfig {
	c := &E2EConfig{}
	sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
	if err != nil {
		tb.Fatalf("unable to connect to secret manager: %v", err)
	}
	if err := envconfig.ProcessWith(ctx, c, envconfig.OsLookuper(), secrets.Resolver(sm, &secrets.Config{})); err != nil {
		tb.Fatalf("Unable to process environment: %v", err)
	}
	return c
}

// E2ESuite contains E2E test configs and other useful data.
type E2ESuite struct {
	cfg   *E2EConfig
	db    *database.Database
	realm *database.Realm

	adminKey, deviceKey string
}

// NewAdminAPIClient returns an admin API client.
func (s *E2ESuite) NewAdminAPIClient(context.Context, testing.TB) (*AdminClient, error) {
	return NewAdminClient(s.cfg.AdminAPIURL, s.adminKey)
}

// NewAPIClient returns an API client.
func (s *E2ESuite) NewAPIClient(context.Context, testing.TB) (*APIClient, error) {
	return NewAPIClient(s.cfg.APIServerURL, s.deviceKey)
}

// NewE2ESuite returns an E2E test suite.
func NewE2ESuite(tb testing.TB, ctx context.Context) *E2ESuite {
	cfg := NewE2EConfig(tb, ctx)
	db, err := cfg.DBConfig.Load(ctx)
	if err != nil {
		tb.Fatalf("failed to connect to database: %v", err)
	}
	if err := db.Open(ctx); err != nil {
		tb.Fatalf("failed to open database: %v", err)
	}
	tb.Cleanup(func() {
		if err := db.Close(); err != nil {
			tb.Errorf("failed to close db: %v", err)
		}
	})
	randomStr, err := project.RandomString()
	if err != nil {
		tb.Fatalf("failed to generate random string: %v", err)
	}
	realmName := realmNamePrefix + randomStr
	// Create or reuse the existing realm
	realm, err := db.FindRealmByName(realmName)
	if err != nil {
		if !database.IsNotFound(err) {
			tb.Fatalf("error when finding the realm %q: %v", realmName, err)
		}
		realm = database.NewRealmWithDefaults(realmName)
		realm.RegionCode = realmRegionCode
		realm.AllowBulkUpload = true
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			tb.Fatalf("failed to create realm %+v: %v: %v", realm, err, realm.ErrorMessages())
		}
	}

	// Create new API keys
	suffix, err := project.RandomString()
	if err != nil {
		tb.Fatalf("failed to create suffix string for API keys: %v", err)
	}

	adminKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       adminKeyName + suffix,
		APIKeyType: database.APIKeyTypeAdmin,
	}, database.SystemTest)
	if err != nil {
		tb.Fatalf("error trying to create a new Admin API Key: %v", err)
	}

	deviceKey, err := realm.CreateAuthorizedApp(db, &database.AuthorizedApp{
		Name:       deviceKeyName + suffix,
		APIKeyType: database.APIKeyTypeDevice,
	}, database.SystemTest)
	if err != nil {
		tb.Fatalf("error trying to create a new Device API Key: %v", err)
	}

	return &E2ESuite{
		cfg:       cfg,
		db:        db,
		realm:     realm,
		adminKey:  adminKey,
		deviceKey: deviceKey,
	}
}
