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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func TestUser_BeforeSave(t *testing.T) {
	t.Parallel()

	cases := []struct {
		structField string
		field       string
	}{
		{"Email", "email"},
		{"Name", "name"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			exerciseValidation(t, &User{}, tc.structField, tc.field)
		})
	}
}

func TestUser_Lifecycle(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	email := "dr@example.com"
	user := User{
		Email:       email,
		Name:        "Dr Example",
		SystemAdmin: false,
	}

	if err := db.SaveUser(&user, SystemTest); err != nil {
		t.Fatalf("error creating user: %v", err)
	}

	// Find user by ID
	{
		got, err := db.FindUser(user.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := got.ID, user.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}

	// Find user by email
	{
		got, err := db.FindUserByEmail(email)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := got.ID, user.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.Email, user.Email; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.Name, user.Name; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		if got, want := got.SystemAdmin, user.SystemAdmin; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}

	// Update an attribute
	user.SystemAdmin = true
	if err := db.SaveUser(&user, SystemTest); err != nil {
		t.Fatal(err)
	}

	// Update password changed
	now := time.Now().UTC()
	now = now.Truncate(time.Second) // db loses nanoseconds
	if err := db.PasswordChanged(email, now); err != nil {
		t.Fatalf("error updating password changed time: %v", err)
	}

	// Verify updated attribute saved
	{
		got, err := db.FindUserByEmail(email)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := got.SystemAdmin, true; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		if got, want := got.PasswordChanged().Unix(), now.Unix(); got != want {
			t.Errorf("expected %#v to be %#v (diff: %#v)", got, want, got-want)
		}
	}
}

func TestDatabase_PurgeUsers(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	email := "purge@example.com"
	user := &User{
		Email:       email,
		Name:        "Dr Delete",
		SystemAdmin: true,
	}
	if err := db.SaveUser(user, System); err != nil {
		t.Fatalf("error creating user: %v", err)
	}
	expectExists(t, db, user.ID)

	// is admin
	if _, err := db.PurgeUsers(time.Duration(0)); err != nil {
		t.Fatal(err)
	}
	expectExists(t, db, user.ID)

	// Update an attribute
	user.SystemAdmin = false
	if err := db.SaveUser(user, SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(db, realm, rbac.LegacyRealmAdmin, SystemTest); err != nil {
		t.Fatal(err)
	}

	// has a realm
	if _, err := db.PurgeUsers(time.Duration(0)); err != nil {
		t.Fatal(err)
	}

	// remove realm
	if err := user.DeleteFromRealm(db, realm, SystemTest); err != nil {
		t.Fatal(err)
	}

	// not old enough
	if _, err := db.PurgeUsers(time.Hour); err != nil {
		t.Fatal(err)
	}
	expectExists(t, db, user.ID)

	// should delete now
	if _, err := db.PurgeUsers(time.Duration(0)); err != nil {
		t.Fatal(err)
	}

	// Find user by ID - Expect deleted
	{
		got, err := db.FindUser(user.ID)
		if err != nil && !IsNotFound(err) {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("expected user to be deleted, got: %#v", got)
		}
	}
}

func TestUser_DeleteFromRealm(t *testing.T) {
	t.Parallel()

	t.Run("updates_time", func(t *testing.T) {
		db, _ := testDatabaseInstance.NewDatabase(t, nil)

		realm := NewRealmWithDefaults("test")
		if err := db.SaveRealm(realm, SystemTest); err != nil {
			t.Fatal(err)
		}

		email := "purge@example.com"
		user := &User{
			Email: email,
			Name:  "Dr Delete",
		}
		if err := db.SaveUser(user, SystemTest); err != nil {
			t.Fatal(err)
		}

		// Add to realm
		if err := user.AddToRealm(db, realm, rbac.LegacyRealmAdmin, SystemTest); err != nil {
			t.Fatal(err)
		}

		got, err := db.FindUser(user.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := got.ID, user.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		time.Sleep(time.Second) // in case this executes in under a nanosecond.

		originalTime := got.Model.UpdatedAt
		if err := user.DeleteFromRealm(db, realm, SystemTest); err != nil {
			t.Fatal(err)
		}

		got, err = db.FindUser(user.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := got.ID, user.ID; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
		// Assert that the user time was updated.
		if originalTime == got.Model.UpdatedAt {
			t.Errorf("expected user time to be updated. Got %#v", originalTime.Format(time.RFC3339))
		}
	})
}

func expectExists(t *testing.T, db *Database, id uint) {
	got, err := db.FindUser(id)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := got.ID, id; got != want {
		t.Errorf("expected %#v to be %#v", got, want)
	}
}

func TestUserNotFound(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	_, err := db.FindUserByEmail("fake@user.com")
	if err == nil {
		t.Fatal("expected error")
	}

	if !IsNotFound(err) {
		t.Errorf("expected %#v to be %#v", err, "not found")
	}
}
