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
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSMSConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	// Create realm
	realmName := t.Name()
	realm, err := db.CreateRealm(realmName)
	if err != nil {
		t.Fatalf("unable to cerate test realm: %v", err)
	}

	// Create SMS config
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

	// Get the config
	gotSMSConfig, err := realm.SMSConfig()
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(gotSMSConfig, smsConfig, approxTime, cmpopts.IgnoreUnexported(SMSConfig{}, Realm{})); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestSMSProvider(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("test-sms-realm-1")
	if err != nil {
		t.Fatalf("realm create failed: %v", err)
	}

	provider, err := realm.SMSProvider()
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

	provider, err = realm.SMSProvider()
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}
}
