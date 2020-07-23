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
	"errors"
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestSMSConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := NewTestDatabase(t)

	// Create a secret manager.
	sm := InMemorySecretManager(map[string]string{
		"my-secret-ref": "def456",
	})
	db.secretManager = sm

	smsConfig := SMSConfig{
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "totally-not-valid", // invalid ref, test error propagation
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	// Lookup config, this should fail because the secret is invalid.
	_, err := db.FindSMSConfig(ctx, "")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrSecretNotExist) {
		t.Errorf("expected %v to be %v", err, ErrSecretNotExist)
	}

	// Delete config.
	if err := db.DeleteSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	// Create valid config.
	smsConfig.TwilioAuthToken = "my-secret-ref"
	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	// Lookup config.
	result, err := db.FindSMSConfig(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	// Secret was resolved.
	if got, want := result.TwilioAuthToken, "def456"; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}

func TestGetSMSProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := NewTestDatabase(t)

	provider, err := db.GetSMSProvider(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	smsConfig := SMSConfig{
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "my-secret-ref",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	// Ensure nil was cached
	provider, err = db.GetSMSProvider(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}
}
