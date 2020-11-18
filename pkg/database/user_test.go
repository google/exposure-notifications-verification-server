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
)

func TestUserLifecycle(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	email := "dr@example.com"
	user := User{
		Email:       email,
		Name:        "Dr Example",
		SystemAdmin: false,
	}

	if err := db.SaveUser(&user, System); err != nil {
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
	if err := db.SaveUser(&user, System); err != nil {
		t.Fatal(err)
	}

	// Update password changed
	now := time.Now().UTC()
	now = now.Truncate(time.Second) // db loses nanos
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

		if got, want := got.PasswordChanged(), now; got != want {
			t.Errorf("expected %#v to be %#v", got.String(), want.String())
		}
	}
}

func TestPurgeUsers(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	email := "purge@example.com"
	user := User{
		Email:       email,
		Name:        "Dr Delete",
		SystemAdmin: true,
	}

	if err := db.SaveUser(&user, System); err != nil {
		t.Fatalf("error creating user: %v", err)
	}
	expectExists(t, db, user.ID)

	db.PurgeUsers(time.Duration(0)) // is admin
	expectExists(t, db, user.ID)

	// Update an attribute
	user.SystemAdmin = false
	realm := NewRealmWithDefaults("test")
	user.AddRealm(realm)
	if err := db.SaveUser(&user, System); err != nil {
		t.Fatal(err)
	}
	db.PurgeUsers(time.Duration(0)) // has a realm

	user.RemoveRealm(realm)
	if err := db.SaveUser(&user, System); err != nil {
		t.Fatal(err)
	}

	db.PurgeUsers(time.Hour) // not old enough
	expectExists(t, db, user.ID)

	db.PurgeUsers(time.Duration(0))

	// Find user by ID - Expect deleted
	{
		got, err := db.FindUser(user.ID)
		if err != nil && !IsNotFound(err) {
			t.Fatalf("Expected user to be deleted. got: %v", err)
		}
		if got != nil {
			t.Fatalf("Expected user to be deleted. got: %v", got)
		}
	}
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

	db := NewTestDatabase(t)

	_, err := db.FindUserByEmail("fake@user.com")
	if err == nil {
		t.Fatal("expected error")
	}

	if !IsNotFound(err) {
		t.Errorf("expected %#v to be %#v", err, "not found")
	}
}
