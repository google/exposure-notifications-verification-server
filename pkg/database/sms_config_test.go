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
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestSMSConfig_Validate(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		smsConfig *SMSConfig
		err       string
	}{
		{
			name: "missing token",
			smsConfig: &SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     sms.ProviderType("TWILIO"),
				TwilioAccountSid: "abc123",
			},
			err: "validation failed",
		},
		{
			name: "invalid phone short",
			smsConfig: &SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     sms.ProviderType("TWILIO"),
				TwilioAccountSid: "abc123",
				TwilioAuthToken:  "def123",
				TwilioFromNumber: "nope",
			},
			err: "validation failed",
		},
		{
			name: "invalid phone long",
			smsConfig: &SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     sms.ProviderType("TWILIO"),
				TwilioAccountSid: "abc123",
				TwilioAuthToken:  "def123",
				TwilioFromNumber: "+invalid",
			},
			err: "validation failed",
		},
		{
			name: "invalid messaging group",
			smsConfig: &SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     sms.ProviderType("TWILIO"),
				TwilioAccountSid: "abc123",
				TwilioAuthToken:  "def123",
				TwilioFromNumber: "MGinvalid",
			},
			err: "validation failed",
		},
		{
			name: "default invalid",
			smsConfig: &SMSConfig{
				RealmID:          realm.ID,
				ProviderType:     sms.ProviderType("TWILIO"),
				TwilioAccountSid: "abc123",
				TwilioAuthToken:  "def123",
				TwilioFromNumber: "invalid",
			},
			err: "validation failed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := db.SaveSMSConfig(tc.smsConfig)

			if tc.err == "" {
				if err != nil {
					t.Error(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("expected error. got %w, want %s", err, tc.err)
			}
		})
	}
}

func TestSMSConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Initial config should be nil
	{
		got, err := realm.SMSConfig(db)
		if !IsNotFound(err) {
			t.Errorf("expected %#v to be %#v", err, "not found")
		}
		if got != nil {
			t.Errorf("expected %#v to be %#v", got, nil)
		}
	}

	if has, err := realm.HasSMSConfig(db); err != nil {
		t.Fatal(err)
	} else if has {
		t.Fatal("expected no SMS config for realm")
	}

	// Create SMS config on the realm
	smsConfig := &SMSConfig{
		RealmID:          realm.ID,
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "def123",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	// Get the realm to verify SMS configs are NOT preloaded
	realm, err = db.FindRealm(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	if has, err := realm.HasSMSConfig(db); err != nil {
		t.Fatal(err)
	} else if !has {
		t.Fatal("expected realm to have SMS config")
	}

	// Load the SMS config
	{
		got, err := realm.SMSConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("expected SMSConfig, got %#v", got)
		}

		if got, want := got.ID, smsConfig.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.RealmID, smsConfig.RealmID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.ProviderType, smsConfig.ProviderType; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.TwilioAccountSid, smsConfig.TwilioAccountSid; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.TwilioAuthToken, smsConfig.TwilioAuthToken; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.TwilioFromNumber, smsConfig.TwilioFromNumber; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}

	// Update value
	smsConfig.TwilioAuthToken = "banana123"
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	// Read back updated value
	{
		got, err := realm.SMSConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("expected SMSConfig, got %#v", got)
		}

		if got, want := got.TwilioAuthToken, "banana123"; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}
}

func TestSMSProvider(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	provider, err := realm.SMSProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	smsConfig := &SMSConfig{
		RealmID:          realm.ID,
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "my-secret-ref",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = realm.SMSProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}
}

func TestSystemSMSProvider(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("test-email-realm-1")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	provider, err := db.SystemSMSConfig()
	if !IsNotFound(err) {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	smsConfig := &SMSConfig{
		RealmID:          realm.ID,
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "my-secret-ref",
		TwilioFromNumber: "+11234567890",
		IsSystem:         true,
	}
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = db.SystemSMSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}

	// Empty creds to delete
	smsConfig.TwilioAccountSid = ""
	smsConfig.TwilioAuthToken = ""
	smsConfig.TwilioFromNumber = ""
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = db.SystemSMSConfig()
	if !IsNotFound(err) {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}
}
