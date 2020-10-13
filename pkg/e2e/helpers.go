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

// Package e2e contains E2E tests and utility.
package e2e

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/integration"
	"github.com/sethvargo/go-envconfig"
)

const (
	realmName       = "e2e-test-realm"
	realmRegionCode = "test"
	adminKeyName    = "e2e-admin-key"
	deviceKeyName   = "e2e-device-key"
)

// Suite contains E2E test configs and other useful data.
type Suite struct {
	cfg   *Config
	db    *database.Database
	realm *database.Realm

	adminKey, deviceKey string
}

func (s *Suite) NewAdminAPIClient() *integration.AdminClient {
	return integration.NewAdminClient(s.cfg.AdminAPIURL, s.adminKey)
}

func (s *Suite) NewAPIClient() *integration.APIClient {
	return integration.NewAPIClient(s.cfg.APIServerURL, s.deviceKey)
}

func NewE2ESuite(tb testing.TB, ctx context.Context) *Suite {
	cfg := initConfig(tb, ctx)
	db, err := cfg.DBConfig.Load(ctx)
	if err != nil {
		tb.Fatalf("failed to connect to database: %v", err)
	}

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

	return &Suite{
		cfg:       cfg,
		db:        db,
		realm:     realm,
		adminKey:  adminKey,
		deviceKey: deviceKey,
	}
}

type Config struct {
	DBName       string `env:"DB_NAME"`
	APIServerURL string `env:"APISERVER_URL"`
	AdminAPIURL  string `env:"ADMINAPI_URL"`
	ProjectID    string `env:"PROJECT_ID"`
	DBConfig     *database.Config
}

func initConfig(tb testing.TB, ctx context.Context) *Config {
	c := &Config{}
	sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
	if err != nil {
		tb.Fatalf("unable to connect to secret manager: %v", err)
	}
	if err := envconfig.ProcessWith(ctx, c, envconfig.OsLookuper(), secrets.Resolver(sm, &secrets.Config{})); err != nil {
		tb.Fatalf("Unable to process environment: %v", err)
	}
	return c
}

func randomString() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04x", n), nil
}
