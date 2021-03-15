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
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	keydatabase "github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/enkstest"
)

// IntegrationSuite encompasses a local API server and Admin API server for
// testing. Both servers run in-memory on the local machine.
type IntegrationSuite struct {
	AdminAPIAddress string
	AdminAPIKey     string

	APIServerAddress string
	APIServerKey     string

	KeyServerAddress string
	KeyServerData    *enkstest.BootstrapResponse

	ENXRedirectAddress string
}

// NewIntegrationSuite creates a new test suite for local integration testing.
func NewIntegrationSuite(tb testing.TB, testDatabaseInstance *database.TestInstance, testKeyServerDatabaseInstance *keydatabase.TestInstance) *IntegrationSuite {
	tb.Helper()

	ctx := project.TestContext(tb)

	keyServer := enkstest.NewServer(tb, testKeyServerDatabaseInstance)
	keyServerData, err := enkstest.Bootstrap(ctx, keyServer.Env)
	if err != nil {
		tb.Fatalf("failed to bootstrap key server: %v", err)
	}

	adminAPIServerConfig := NewAdminAPIServerConfig(tb, testDatabaseInstance)
	apiServerConfig := NewAPIServerConfig(tb, testDatabaseInstance)
	enxRedirectConfig := NewENXRedirectServerConfig(tb, testDatabaseInstance)

	// Point everything at the same database, cacher, and key manager.
	adminAPIServerConfig.Database = apiServerConfig.Database
	adminAPIServerConfig.BadDatabase = apiServerConfig.BadDatabase
	adminAPIServerConfig.Cacher = apiServerConfig.Cacher
	adminAPIServerConfig.KeyManager = apiServerConfig.KeyManager
	adminAPIServerConfig.RateLimiter = apiServerConfig.RateLimiter
	adminAPIServerConfig.Renderer = apiServerConfig.Renderer

	enxRedirectConfig.Database = apiServerConfig.Database
	enxRedirectConfig.BadDatabase = apiServerConfig.BadDatabase
	enxRedirectConfig.Cacher = apiServerConfig.Cacher
	enxRedirectConfig.Renderer = apiServerConfig.Renderer

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
	realm.UseRealmCertificateKey = true
	realm.CertificateIssuer = "test-iss"
	realm.CertificateAudience = "test-aud"
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		tb.Fatalf("failed to update to realm certificates: %v", err)
	}

	if _, err := realm.CreateSigningKeyVersion(ctx, db, database.SystemTest); err != nil {
		tb.Fatalf("failed to create certificate signing key version: %v", err)
	}

	certificateSigningKey, err := realm.CurrentSigningKey(db)
	if err != nil {
		tb.Fatalf("failed to get current signing key: %v", err)
	}

	signer, err := apiServerConfig.KeyManager.NewSigner(ctx, certificateSigningKey.ManagedKeyID())
	if err != nil {
		tb.Fatalf("failed to get certificate signer: %v", err)
	}

	x509Bytes, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		tb.Fatalf("failed to marshal certificate signer public key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509Bytes})

	// Insert the verification server's public key into the key server as a
	// recognized health authority.
	updatedHealthAuthorityKey := keyServerData.HealthAuthorityKey
	updatedHealthAuthorityKey.Version = certificateSigningKey.GetKID()
	updatedHealthAuthorityKey.PublicKeyPEM = string(pemBytes)
	if err := keyServer.AddHealthAuthorityKey(ctx, keyServerData.HealthAuthority, updatedHealthAuthorityKey); err != nil {
		tb.Fatalf("failed to update health authority key: %v", err)
	}

	// Configure SMS
	if project.SkipE2ESMS {
		tb.Logf("🚧 🚧 Skipping sms tests (missing TWILIO_ACCOUNT_SID/TWILIO_AUTH_TOKEN)")

		// Also disable authenticated sms
		resp.Realm.UseAuthenticatedSMS = false
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			tb.Fatalf("failed to update realm: %v", err)
		}
	} else {
		twilioAccountSid := os.Getenv("TWILIO_ACCOUNT_SID")
		if twilioAccountSid == "" {
			tb.Fatalf("missing TWILIO_ACCOUNT_SID")
		}
		twilioAuthToken := os.Getenv("TWILIO_AUTH_TOKEN")
		if twilioAuthToken == "" {
			tb.Fatalf("missing TWILIO_AUTH_TOKEN")
		}

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
	enxRedirectServer := enxRedirectConfig.NewServer(tb)

	return &IntegrationSuite{
		AdminAPIAddress: "http://" + adminAPIServer.Server.Addr(),
		AdminAPIKey:     resp.AdminAPIKey,

		APIServerAddress: "http://" + apiServer.Server.Addr(),
		APIServerKey:     resp.DeviceAPIKey,

		KeyServerAddress: "http://" + keyServer.Server.Addr(),
		KeyServerData:    keyServerData,

		ENXRedirectAddress: "http://" + enxRedirectServer.Server.Addr(),
	}
}
