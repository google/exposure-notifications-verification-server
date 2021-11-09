// Copyright 2021 the Exposure Notifications Verification Server authors
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

package cleanup

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/jinzhu/gorm"
)

func TestHandleCleanup(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	deletedAt := time.Now().UTC().Add(-8760 * time.Hour)

	keyManager := keys.TestKeyManager(t)
	keyManagerSigner, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		t.Fatal("kms cannot manage signing keys")
	}
	tokenSigningKey := keys.TestSigningKey(t, keyManager)

	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	config := &config.CleanupConfig{
		TokenSigning: config.TokenSigningConfig{
			TokenSigningKey: tokenSigningKey,
		},
		SigningTokenKeyMaxAge: 1 * time.Second,
	}

	t.Run("api_keys", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		c := New(config, db, keyManagerSigner, h)

		authApp := &database.AuthorizedApp{
			Name: "appy",
			Model: gorm.Model{
				DeletedAt: &deletedAt,
			},
		}
		if _, err := realm.CreateAuthorizedApp(db, authApp, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		apps, _, err := realm.ListAuthorizedApps(db, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(apps), 0; got != want {
			t.Errorf("got %d apps, expected %d", got, want)
		}
	})

	t.Run("verification_codes", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		c := New(config, db, keyManagerSigner, h)

		code := &database.VerificationCode{
			Code:          "12345678",
			LongCode:      "12345678901122334455",
			TestType:      "confirmed",
			ExpiresAt:     time.Now().UTC().Add(24 * time.Hour),
			LongExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}
		if err := realm.SaveVerificationCode(db, code); err != nil {
			t.Fatal(err)
		}
		if err := db.RawDB().Model(code).UpdateColumns(&database.VerificationCode{
			ExpiresAt:     time.Now().UTC().Add(-24 * time.Hour),
			LongExpiresAt: time.Now().UTC().Add(-24 * time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		var codes []*database.VerificationCode
		if err := db.RawDB().
			Unscoped().
			Model(&database.VerificationCode{}).
			Find(&codes).
			Error; err != nil {
			t.Fatal(err)
		}
		if got, want := len(codes), 0; got != want {
			t.Errorf("got %d codes, expected %d", got, want)
		}
	})

	t.Run("verification_tokens", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c := New(config, db, keyManagerSigner, h)

		token := &database.Token{
			TestType:  "confirmed",
			ExpiresAt: time.Now().UTC().Add(2 * time.Second),
		}
		if err := db.RawDB().Save(token).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.RawDB().Model(token).UpdateColumns(&database.Token{
			ExpiresAt: time.Now().UTC().Add(-24 * time.Hour),
		}).Error; err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		var tokens []*database.Token
		if err := db.RawDB().
			Unscoped().
			Model(&database.Token{}).
			Find(&tokens).
			Error; err != nil {
			t.Fatal(err)
		}
		if got, want := len(tokens), 0; got != want {
			t.Errorf("got %d tokens, expected %d", got, want)
		}
	})

	t.Run("orphaned_memberships", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		c := New(config, db, keyManagerSigner, h)

		// Create users in the realm with the provided permissions
		user1 := &database.User{
			Email: "one@example.com",
			Name:  "one",
		}
		if err := db.SaveUser(user1, database.SystemTest); err != nil {
			t.Fatal(err)
		}
		membership1 := &database.Membership{
			UserID:      user1.ID,
			RealmID:     realm.ID,
			Permissions: 0,
		}
		if err := db.SaveMembership(membership1, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		user2 := &database.User{
			Email: "two@example.com",
			Name:  "two",
		}
		if err := db.SaveUser(user2, database.SystemTest); err != nil {
			t.Fatal(err)
		}
		membership2 := &database.Membership{
			UserID:      user2.ID,
			RealmID:     realm.ID,
			Permissions: rbac.CodeIssue,
		}
		if err := db.SaveMembership(membership2, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		{
			memberships, _, err := realm.ListMemberships(db, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := len(memberships), 2; got != want {
				t.Errorf("got %d memberships, expected %d: %#v", got, want, memberships)
			}
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		{
			memberships, _, err := realm.ListMemberships(db, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Only memberships with 0 permissions should be deleted.
			if got, want := len(memberships), 1; got != want {
				t.Errorf("got %d memberships, expected %d: %#v", got, want, memberships)
			}
		}
	})

	t.Run("mobile_apps", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		c := New(config, db, keyManagerSigner, h)

		app := &database.MobileApp{
			Name:    "Appy",
			RealmID: realm.ID,
			URL:     "https://example.com",
			OS:      database.OSTypeIOS,
			AppID:   "a.b.c.d",
			Model: gorm.Model{
				DeletedAt: &deletedAt,
			},
		}
		if err := db.SaveMobileApp(app, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		apps, _, err := realm.ListMobileApps(db, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(apps), 0; got != want {
			t.Errorf("got %d apps, expected %d", got, want)
		}
	})

	t.Run("audits", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c := New(config, db, keyManagerSigner, h)

		audit := database.BuildAuditEntry(database.SystemTest, "read", database.SystemTest, 0)
		if err := db.RawDB().Save(audit).Error; err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		audits, _, err := db.ListAudits(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(audits), 0; got != want {
			t.Errorf("got %d audits, expected %d", got, want)
		}
	})

	t.Run("users", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		c := New(config, db, keyManagerSigner, h)

		user := &database.User{
			Name:  "User",
			Email: "user@example.com",
			Model: gorm.Model{
				CreatedAt: deletedAt,
				UpdatedAt: deletedAt,
			},
		}
		if err := db.SaveUser(user, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		users, _, err := db.ListUsers(nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(users), 1; got != want {
			// There's still the default user from the migration
			t.Errorf("got %d users, expected %d", got, want)
		}
	})

	t.Run("chaff_events", func(t *testing.T) {
		t.Parallel()

		db, _ := testDatabaseInstance.NewDatabase(t, nil)
		realm, err := db.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}
		if err := realm.RecordChaffEvent(db, deletedAt); err != nil {
			t.Fatal(err)
		}

		c := New(config, db, keyManagerSigner, h)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodGet, "/", nil)
		c.HandleCleanup().ServeHTTP(w, r)

		events, err := realm.ListChaffEvents(db)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if event.Present {
				t.Errorf("expected event to not be present: %#v", event)
			}
		}
	})
}
