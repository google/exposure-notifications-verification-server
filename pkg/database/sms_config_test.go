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

package database

import (
	"context"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSMSConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := NewTestDatabase(t)

	realmName := t.Name()
	var wantSMSConfig *SMSConfig
	{
		realm, err := db.CreateRealm(realmName)
		if err != nil {
			t.Fatalf("unable to cerate test realm: %v", err)
		}

		// Create a secret manager.
		sm, err := secrets.NewInMemoryFromMap(ctx, map[string]string{
			"my-secret-ref": "def456",
		})
		if err != nil {
			t.Fatal(err)
		}
		db.secretManager = sm

		realm.SMSConfig = &SMSConfig{
			ProviderType:     sms.ProviderType("TWILIO"),
			TwilioAccountSid: "abc123",
			TwilioAuthToken:  "totally-not-valid", // invalid ref, test error propagation
			TwilioFromNumber: "+11234567890",
		}
		if err := db.SaveRealm(realm); err != nil {
			t.Fatal(err)
		}
		wantSMSConfig = realm.SMSConfig
	}

	{
		realm, err := db.GetRealmByName(realmName)
		if err != nil {
			t.Fatalf("failed to read back realm: %v", err)
		}

		// Get the SMSConfig.
		smsConfig, err := realm.GetSMSConfig(ctx, db)
		if err != nil {
			t.Fatalf("error getting sms config: %v", err)
		}

		if diff := cmp.Diff(wantSMSConfig, smsConfig, approxTime, cmpopts.IgnoreUnexported(SMSConfig{})); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}

		// Attempt to get the SMS provider, but secret resolution should fail.
		_, err = smsConfig.GetSMSProvider(ctx, db)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected %v to be %v", err, "does not exist")
		}

		// Delete config.
		if err := db.DeleteSMSConfig(smsConfig); err != nil {
			t.Fatal(err)
		}
	}

	{
		realm, err := db.GetRealmByName(realmName)
		if err != nil {
			t.Fatalf("failed to read back realm: %v", err)
		}

		// Get the SMSConfig.
		smsConfig, err := realm.GetSMSConfig(ctx, db)
		if err != nil {
			t.Fatalf("error getting sms config: %v", err)
		}
		if smsConfig != nil {
			t.Fatalf("read back deleted sms config, unexpected")
		}
		// Create valid config.
		smsConfig = &SMSConfig{
			ProviderType:     sms.ProviderType("TWILIO"),
			TwilioAccountSid: "abc123",
			TwilioAuthToken:  "my-secret-ref", // Valid secret
			TwilioFromNumber: "+11234567890",
		}
		if err := db.SaveRealm(realm); err != nil {
			t.Fatalf("error saving realm/sms config: %v", err)
		}

		// Get the provider.
		result, err := smsConfig.GetSMSProvider(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil {
			t.Fatalf("sms provider was nil")
		}

		// Secret was resolved.
		if got, want := smsConfig.twilioAuthSecret, "def456"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}

func TestGetSMSProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("test-sms-realm-1")
	if err != nil {
		t.Fatalf("realm create failed: %v", err)
	}

	smsConfig, err := realm.GetSMSConfig(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if smsConfig != nil {
		t.Errorf("expected %v to be %v", smsConfig, nil)
	}

	realm.SMSConfig = &SMSConfig{
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "my-secret-ref",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveRealm(realm); err != nil {
		t.Fatal(err)
	}

	provider, err := realm.SMSConfig.GetSMSProvider(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}
}
