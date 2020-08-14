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
)

func TestUserLifecycle(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	email := "dr@example.com"
	user := User{
		Email: email,
		Name:  "Dr Example",
		Admin: false,
	}

	if err := db.SaveUser(&user); err != nil {
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
		if got, want := got.Admin, user.Admin; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	}

	// Update an attribute
	user.Admin = true
	if err := db.SaveUser(&user); err != nil {
		t.Fatal(err)
	}

	// Verify updated attribute saved
	{
		got, err := db.FindUserByEmail(email)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := got.Admin, true; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
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
