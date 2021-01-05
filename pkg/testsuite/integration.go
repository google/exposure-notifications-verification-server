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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

const (
	realmNamePrefix = "test-realm"
	realmRegionCode = "test"
	adminKeyName    = "integration-admin-key"
	deviceKeyName   = "integration-device-key"
)

// IntegrationSuite encompasses a local API server and Admin API server for
// testing. Both servers run in-memory on the local machine.
type IntegrationSuite struct {
	adminAPIServerClient *clients.AdminAPIServerClient
	apiServerClient      *clients.APIServerClient
}

// NewIntegrationSuite creates a new test suite for local integration testing.
func NewIntegrationSuite(tb testing.TB) *IntegrationSuite {
	tb.Helper()

	testDatabaseInstance, err := database.NewTestInstance()
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := testDatabaseInstance.Close(); err != nil {
			tb.Fatal(err)
		}
	})

	adminAPIServerConfig := envstest.NewAdminAPIServerConfig(tb, testDatabaseInstance)
	apiServerConfig := envstest.NewAPIServerConfig(tb, testDatabaseInstance)

	// Point everything at the same database, cacher, and key manager.
	apiServerConfig.Database = adminAPIServerConfig.Database
	apiServerConfig.Cacher = adminAPIServerConfig.Cacher
	apiServerConfig.RateLimiter = adminAPIServerConfig.RateLimiter

	db := adminAPIServerConfig.Database

	realm, err := db.FindRealm(1)
	if err != nil {
		tb.Fatal(err)
	}
	realm.RegionCode = realmRegionCode
	realm.AllowBulkUpload = true
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		tb.Fatal(err)
	}

	adminAPIServer := adminAPIServerConfig.NewServer(tb)
	apiServer := apiServerConfig.NewServer(tb)

	adminAPIServerClient := adminAPIServer.NewAdminAPIServerClient(tb)
	apiServerClient := apiServer.NewAPIServerClient(tb)

	return &IntegrationSuite{
		adminAPIServerClient: adminAPIServerClient,
		apiServerClient:      apiServerClient,
	}
}

// AdminAPIServerClient returns the Admin API client.
func (i *IntegrationSuite) AdminAPIServerClient() *clients.AdminAPIServerClient {
	return i.adminAPIServerClient
}

// APIServerClient returns the API server client.
func (i *IntegrationSuite) APIServerClient() *clients.APIServerClient {
	return i.apiServerClient
}
