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

	"github.com/google/exposure-notifications-verification-server/pkg/email"
)

func TestEmailConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Create realm
	realmName := t.Name()
	realm, err := db.CreateRealm(realmName)
	if err != nil {
		t.Fatalf("unable to cerate test realm: %v", err)
	}

	// Initial config should be nil
	{
		got, err := realm.EmailConfig(db)
		if !IsNotFound(err) {
			t.Errorf("expected %#v to be %#v", err, "not found")
		}
		if got != nil {
			t.Errorf("expected %#v to be %#v", got, nil)
		}
	}

	// Create email config on the realm
	emailConfig := &EmailConfig{
		RealmID:      realm.ID,
		ProviderType: email.ProviderType(email.ProviderTypeSMTP),
		SMTPAccount:  "noreply@sendemails.meh",
		SMTPPassword: "my-secret-ref",
		SMTPHost:     "smtp.sendemails.meh",
	}
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	// Get the realm to verify email configs are NOT preloaded
	realm, err = db.FindRealm(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Load the email config
	{
		got, err := realm.EmailConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("expected emailConfig, got %#v", got)
		}

		if got, want := got.ID, emailConfig.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.RealmID, emailConfig.RealmID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.ProviderType, emailConfig.ProviderType; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.SMTPAccount, emailConfig.SMTPAccount; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.SMTPPassword, emailConfig.SMTPPassword; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.SMTPHost, emailConfig.SMTPHost; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}

	// Update value
	emailConfig.SMTPPassword = "banana123"
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	// Read back updated value
	{
		got, err := realm.EmailConfig(db)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatalf("expected EmailConfig, got %#v", got)
		}

		if got, want := got.SMTPPassword, "banana123"; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}
}

func TestEmailProvider(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.CreateRealm("test-email-realm-1")
	if err != nil {
		t.Fatalf("realm create failed: %v", err)
	}

	provider, err := realm.EmailProvider(db)
	if !IsNotFound(err) {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	emailConfig := &EmailConfig{
		RealmID:      realm.ID,
		ProviderType: email.ProviderType(email.ProviderTypeSMTP),
		SMTPAccount:  "noreply@sendemails.meh",
		SMTPPassword: "my-secret-ref",
		SMTPHost:     "smtp.sendemails.meh",
	}
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = realm.EmailProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}
}
