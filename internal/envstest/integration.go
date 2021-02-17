// Copyright 2021 Google LLC
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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// IntegrationSuite encompasses a local API server and Admin API server for
// testing. Both servers run in-memory on the local machine.
type IntegrationSuite struct {
	adminAPIServerClient *clients.AdminAPIServerClient
	apiServerClient      *clients.APIServerClient
}

// NewIntegrationSuite creates a new test suite for local integration testing.
func NewIntegrationSuite(tb testing.TB, testDatabaseInstance *database.TestInstance) *IntegrationSuite {
	tb.Helper()

	ctx := project.TestContext(tb)

	adminAPIServerConfig := NewAdminAPIServerConfig(tb, testDatabaseInstance)
	apiServerConfig := NewAPIServerConfig(tb, testDatabaseInstance)

	// Point everything at the same database, cacher, and key manager.
	adminAPIServerConfig.Database = apiServerConfig.Database
	adminAPIServerConfig.Cacher = apiServerConfig.Cacher
	adminAPIServerConfig.KeyManager = apiServerConfig.KeyManager
	adminAPIServerConfig.RateLimiter = apiServerConfig.RateLimiter

	db := adminAPIServerConfig.Database

	resp, err := Bootstrap(ctx, db)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := resp.Cleanup(); err != nil {
			tb.Fatal(err)
		}
	})

	adminAPIServer := adminAPIServerConfig.NewServer(tb)
	apiServer := apiServerConfig.NewServer(tb)

	adminAPIServerClient, err := clients.NewAdminAPIServerClient("http://"+adminAPIServer.Server.Addr(), resp.AdminAPIKey)
	if err != nil {
		tb.Fatal(err)
	}

	apiServerClient, err := clients.NewAPIServerClient("http://"+apiServer.Server.Addr(), resp.DeviceAPIKey)
	if err != nil {
		tb.Fatal(err)
	}

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
