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
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

func TestAPIKeyType(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant, but it's designed to ensure that
	// the exact values for existing types remain unchanged.
	cases := []struct {
		t   APIKeyType
		exp int
	}{
		{APIKeyTypeInvalid, -1},
		{APIKeyTypeDevice, 0},
		{APIKeyTypeAdmin, 1},
		{APIKeyTypeStats, 2},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.t.Display(), func(t *testing.T) {
			t.Parallel()

			if got, want := int(tc.t), tc.exp; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestAPIKeyType_Display(t *testing.T) {
	t.Parallel()

	cases := []struct {
		t   APIKeyType
		exp string
	}{
		{APIKeyTypeInvalid, "invalid"},
		{APIKeyTypeDevice, "device"},
		{APIKeyTypeAdmin, "admin"},
		{APIKeyTypeStats, "stats"},
		{1991, "invalid"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%d", tc.t), func(t *testing.T) {
			t.Parallel()

			if got, want := tc.t.Display(), tc.exp; got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestAuthorizedApp_BeforeSave(t *testing.T) {
	t.Parallel()

	t.Run("name", func(t *testing.T) {
		t.Parallel()
		exerciseValidation(t, &AuthorizedApp{}, "Name", "name")
	})

	t.Run("type", func(t *testing.T) {
		t.Parallel()

		{
			var m AuthorizedApp
			m.APIKeyType = -1
			_ = m.BeforeSave(&gorm.DB{})
			if errs := m.ErrorsFor("type"); len(errs) < 1 {
				t.Errorf("expected errors for type")
			}
		}

		{
			var m AuthorizedApp
			m.APIKeyType = 55
			_ = m.BeforeSave(&gorm.DB{})
			if errs := m.ErrorsFor("type"); len(errs) < 1 {
				t.Errorf("expected errors for type")
			}
		}

		{
			var m AuthorizedApp
			m.APIKeyType = 0
			_ = m.BeforeSave(&gorm.DB{})
			if errs := m.ErrorsFor("type"); len(errs) != 0 {
				t.Errorf("expected no errors for type, got %v", errs)
			}
		}
	})
}

func TestAuthorizedApp_Realm(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authorizedApp := &AuthorizedApp{
		Name: "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(db, authorizedApp, SystemTest); err != nil {
		t.Fatal(err)
	}

	gotRealm, err := authorizedApp.Realm(db)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := gotRealm.ID, realm.ID; got != want {
		t.Errorf("Expected %d to be %d", got, want)
	}
}

func TestAuthorizedApp_Stats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authorizedApp := &AuthorizedApp{
		Name: "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(db, authorizedApp, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Ensure graph is contiguous.
	{
		stats, err := authorizedApp.Stats(db)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(stats), 30; got < want {
			t.Errorf("expected stats for %d days, got %d", want, got)
		}
	}
}

func TestDatabase_CreateFindAPIKey(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("foo")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	authApp := &AuthorizedApp{
		Name:       "University System Health Org",
		APIKeyType: APIKeyTypeAdmin,
	}

	apiKey, err := realm.CreateAuthorizedApp(db, authApp, SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.FindAuthorizedAppByAPIKey(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatalf("expected result")
	}

	if _, err := got.Realm(db); err != nil {
		t.Fatalf("expected realm: %v", err)
	}

	if strings.Contains(apiKey, authApp.APIKey) {
		t.Errorf("database API key should be HMACed!")
	}

	if got, want := got.RealmID, authApp.RealmID; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
	if got, want := got.Name, authApp.Name; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
	if got, want := got.APIKey, authApp.APIKey; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
	if got, want := got.APIKeyPreview, authApp.APIKeyPreview; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
	if got, want := got.APIKeyType, authApp.APIKeyType; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
}

func TestDatabase_GenerateAPIKey(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("bar")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	key, err := db.GenerateAPIKey(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.SplitN(key, ".", 3)
	if got, want := len(parts), 3; got != want {
		t.Fatalf("expected %v to be %v", got, want)
	}

	if got, want := parts[1], fmt.Sprintf("%d", realm.ID); got != want {
		t.Fatalf("expected %v to be %v", got, want)
	}
}

func TestDatabase_GenerateVerifyAPIKeySignature(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	apiKey, realmID := "abcd1234", uint64(15)

	key := fmt.Sprintf("%s.%d", apiKey, realmID)
	sig, err := db.GenerateAPIKeySignature(key)
	if err != nil {
		t.Fatal(err)
	}
	key = fmt.Sprintf("%s.%s", key, base64.RawURLEncoding.EncodeToString(sig))

	gotAPIKey, gotRealmID, err := db.VerifyAPIKeySignature(key)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := gotAPIKey, apiKey; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	if got, want := gotRealmID, realmID; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}

func TestDatabase_PurgeAuthorizedApps(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := db.SaveAuthorizedApp(&AuthorizedApp{
			RealmID: 1,
			Name:    fmt.Sprintf("appy%d", i),
			APIKey:  fmt.Sprintf("%d", i),
			Model: gorm.Model{
				DeletedAt: &now,
			},
		}, SystemTest); err != nil {
			t.Fatal(err)
		}
	}

	// Should not purge entries (too young).
	{
		n, err := db.PurgeAuthorizedApps(24 * time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(0); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}

	// Purges entries.
	{
		n, err := db.PurgeAuthorizedApps(1 * time.Nanosecond)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(5); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}
}

func TestAuthorizedApp_Audits(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authorizedApp := &AuthorizedApp{
		Name: "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(db, authorizedApp, SystemTest); err != nil {
		t.Fatal(err)
	}

	authorizedApp.Name = "something else"
	if err := db.SaveAuthorizedApp(authorizedApp, SystemTest); err != nil {
		t.Fatalf("%v, %v", err, authorizedApp.errors)
	}

	audits, _, err := db.ListAudits(&pagination.PageParams{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(audits), 2; got != want {
		t.Errorf("expected %d audits, got %d: %v", want, got, audits)
	}
}
