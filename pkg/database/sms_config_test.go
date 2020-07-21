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
)

func TestSMSConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	smsConfig := SMSConfig{
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "def456",
		TwilioFromNumber: "+11234567890",
	}

	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	got, err := db.FindSMSConfig("")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(smsConfig, *got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	smsConfig.TwilioFromNumber = "+10987654321"
	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	got, err = db.FindSMSConfig("")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(smsConfig, *got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestGetSMSProvider(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	provider, err := db.GetSMSProvider("")
	if err != nil {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	smsConfig := SMSConfig{
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "def456",
		TwilioFromNumber: "+11234567890",
	}
	if err := db.SaveSMSConfig(&smsConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = db.GetSMSProvider("")
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Fatal("expected provider")
	}

	if _, ok := provider.(*sms.Twilio); !ok {
		t.Fatal("expected provider to be twilio")
	}
}
