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
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestUserLifecycle(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	email := "dr@example.com"
	user := User{
		Email:    email,
		Name:     "Dr Example",
		Admin:    false,
		Disabled: false,
	}

	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error creating user: %v", err)
	}

	got, err := db.FindUser(email)
	if err != nil {
		t.Fatalf("error reading user from db: %v", err)
	}

	if diff := cmp.Diff(user, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	user.Admin = true
	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error updating user: %v", err)
	}

	got, err = db.FindUser(email)
	if err != nil {
		t.Fatalf("error reading user from db: %v", err)
	}

	if diff := cmp.Diff(user, *got, approxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestUserNotFound(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	if _, err := db.FindUser("fake@user.com"); err == nil {
		t.Fatalf("expected error, got nil")
	} else if !strings.Contains(err.Error(), "record not found") {
		t.Errorf("wrong error, wanted 'record not found', got '%v'", err)
	}
}

func TestPurgeUsers(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	email := "dr@example.com"
	user := User{
		Email:    email,
		Name:     "Dr Example",
		Admin:    false,
		Disabled: true,
	}

	if err := db.SaveUser(&user); err != nil {
		t.Fatalf("error creating user: %v", err)
	}

	time.Sleep(time.Millisecond * 2)

	count, err := db.PurgeUsers(time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error purging users: %v", err)
	}
	if count != 1 {
		t.Fatalf("wrong row count, want: %v, got: %v", 1, count)
	}

	_, err = db.FindUser(email)
	if err == nil {
		t.Fatalf("expected an error loading nonexistent user.")
	}
}
