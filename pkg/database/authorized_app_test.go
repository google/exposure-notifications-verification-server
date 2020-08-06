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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDatabase_CreateFindAPIKey(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("foo")
	if err != nil {
		t.Fatal(err)
	}

	apiKey, authApp, err := db.CreateAuthorizedApp(realm.ID, "University System Health Org", APIUserTypeAdmin)
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
	if got.Realm == nil {
		t.Fatalf("expected realm to preload")
	}

	if strings.Contains(apiKey, authApp.APIKey) {
		t.Errorf("database API key should be HMACed!")
	}

	// Ignore the preloaded realm on the got AuthorizedApp
	if diff := cmp.Diff(authApp, got, approxTime, cmpopts.IgnoreFields(AuthorizedApp{}, "Realm")); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestDatabase_ListAPIKeys(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("bar")
	if err != nil {
		t.Fatal(err)
	}

	_, authApp1, err := db.CreateAuthorizedApp(realm.ID, "App 1", APIUserTypeAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, authApp2, err := db.CreateAuthorizedApp(realm.ID, "App 2", APIUserTypeDevice)
	if err != nil {
		t.Fatal(err)
	}

	want := []*AuthorizedApp{authApp1, authApp2}
	got, err := db.ListAuthorizedApps(false)

	if err != nil {
		t.Fatalf("error listing apps: %v", err)
	}

	// Ignore the preloaded realm on the got AuthorizedApp
	if diff := cmp.Diff(want, got, approxTime, cmpopts.IgnoreFields(AuthorizedApp{}, "Realm")); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestDatabase_GenerateAPIKey(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("bar")
	if err != nil {
		t.Fatal(err)
	}

	key, err := db.GenerateAPIKey(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.SplitN(key, ":", 3)
	if got, want := len(parts), 3; got != want {
		t.Fatalf("expected %v to be %v", got, want)
	}

	if got, want := parts[1], fmt.Sprintf("%d", realm.ID); got != want {
		t.Fatalf("expected %v to be %v", got, want)
	}
}

func TestDatabase_GenerateVerifyAPIKeySignature(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	apiKey, realmID := "abcd1234", uint(15)

	key := fmt.Sprintf("%s:%d", apiKey, realmID)
	sig, err := db.GenerateAPIKeySignature(key)
	if err != nil {
		t.Fatal(err)
	}
	key = fmt.Sprintf("%s:%x", key, sig)

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
