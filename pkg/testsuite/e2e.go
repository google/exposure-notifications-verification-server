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

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// E2ESuite contains E2E test configs and other useful data.
type E2ESuite struct {
	cfg   *config.E2EConfig
	db    *database.Database
	realm *database.Realm

	adminKey, deviceKey string
}

func (s *E2ESuite) NewAdminAPIClient(context.Context, testing.TB) *AdminClient {
	return NewAdminClient(s.cfg.AdminAPIURL, s.adminKey)
}

func (s *E2ESuite) NewAPIClient(context.Context, testing.TB) *APIClient {
	return NewAPIClient(s.cfg.APIServerURL, s.deviceKey)
}

func NewE2ESuite(tb testing.TB, ctx context.Context) *E2ESuite {
	cfg := config.NewE2EConfig(tb, ctx)
	db, err := cfg.DBConfig.Load(ctx)
	if err != nil {
		tb.Fatalf("failed to connect to database: %v", err)
	}
	if err := db.Open(ctx); err != nil {
		tb.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

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

	return &E2ESuite{
		cfg:       cfg,
		db:        db,
		realm:     realm,
		adminKey:  adminKey,
		deviceKey: deviceKey,
	}
}
