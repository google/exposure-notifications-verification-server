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
	"os"
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
	adminAPIServerConfig.BadDatabase = apiServerConfig.BadDatabase
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

	realm := resp.Realm

	// Configure SMS
	twilioAccountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	twilioAuthToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if twilioAccountSid == "" || twilioAuthToken == "" {
		tb.Logf("🚧 🚧 Skipping sms tests (missing TWILIO_ACCOUNT_SID/TWILIO_AUTH_TOKEN)")

		// Also disable authenticated sms
		resp.Realm.UseAuthenticatedSMS = false
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			tb.Fatalf("failed to update realm: %v", err)
		}
	} else {
		has, err := resp.Realm.HasSMSConfig(db)
		if err != nil {
			tb.Fatalf("failed to check if realm has sms config: %s", err)
		}
		if !has {
			smsConfig := &database.SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     "TWILIO",
				TwilioAccountSid: twilioAccountSid,
				TwilioAuthToken:  twilioAuthToken,
				TwilioFromNumber: "+15005550006",
			}
			if err := db.SaveSMSConfig(smsConfig); err != nil {
				tb.Fatalf("failed to save sms config: %v", err)
			}
		}

		if _, err := realm.CurrentSMSSigningKey(db); err != nil {
			if !database.IsNotFound(err) {
				tb.Fatalf("failed to find current sms signing key: %s", err)
			}

			if _, err := realm.CreateSMSSigningKeyVersion(ctx, db, database.SystemTest); err != nil {
				tb.Fatalf("failed to create signing key: %s", err)
			}

			if _, err = realm.CurrentSMSSigningKey(db); err != nil {
				tb.Fatalf("failed to find current sms signing key after creation: %v", err)
			}
		}
	}

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
