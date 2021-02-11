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

	"github.com/google/exposure-notifications-verification-server/pkg/email"
)

func TestEmailConfig_Validate(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		emailConfig *EmailConfig
		err         string
	}{
		{
			name: "missing password and host",
			emailConfig: &EmailConfig{
				RealmID:      realm.ID,
				ProviderType: email.ProviderTypeSMTP,
				SMTPAccount:  "noreply@sendemails.meh",
			},
			err: "validation failed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := db.SaveEmailConfig(tc.emailConfig)

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

func TestEmailConfig_Lifecycle(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	// Create realm
	realm := NewRealmWithDefaults(t.Name())
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
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
		ProviderType: email.ProviderTypeSMTP,
		SMTPAccount:  "noreply@sendemails.meh",
		SMTPPassword: "my-secret-ref",
		SMTPHost:     "smtp.sendemails.meh",
	}
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	// Get the realm to verify email configs are NOT preloaded
	realm, err := db.FindRealm(realm.ID)
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

	realm := NewRealmWithDefaults("test-email-realm-1")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
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
		ProviderType: email.ProviderTypeSMTP,
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

func TestSystemEmailProvider(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("test-email-realm-1")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	provider, err := db.SystemEmailConfig()
	if !IsNotFound(err) {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}

	emailConfig := &EmailConfig{
		RealmID:      realm.ID,
		ProviderType: email.ProviderTypeSMTP,
		SMTPAccount:  "noreply@sendemails.meh",
		SMTPPassword: "my-secret-ref",
		SMTPHost:     "smtp.sendemails.meh",
		IsSystem:     true,
	}
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = db.SystemEmailConfig()
	if err != nil {
		t.Fatal(err)
	}
	if provider == nil {
		t.Errorf("expected %v to be not nil", provider)
	}

	// Empty creds to delete
	emailConfig.SMTPHost = ""
	emailConfig.SMTPAccount = ""
	emailConfig.SMTPPassword = ""
	if err := db.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	provider, err = db.SystemEmailConfig()
	if !IsNotFound(err) {
		t.Fatal(err)
	}
	if provider != nil {
		t.Errorf("expected %v to be %v", provider, nil)
	}
}
