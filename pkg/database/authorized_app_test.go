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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCreateFindAPIKey(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("foo")
	if err != nil {
		t.Fatalf("error creating realm: %v", err)
	}

	authApp, err := db.CreateAuthorizedApp(realm.ID, "University System Health Org", APIUserTypeAdmin)
	if err != nil {
		t.Fatalf("error creating authorized app: %v", err)
	}

	got, err := db.FindAuthorizedAppByAPIKey(authApp.APIKey)
	if err != nil {
		t.Fatalf("error reading authorized app by api key: %v", err)
	}
	// Ignore the preloaded realm on the got AuthorizedApp
	if diff := cmp.Diff(authApp, got, approxTime, cmpopts.IgnoreFields(AuthorizedApp{}, "Realm")); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestListAPIKeys(t *testing.T) {
	t.Parallel()
	db := NewTestDatabase(t)

	var authApp1, authApp2 *AuthorizedApp
	var err error

	realm, err := db.CreateRealm("bar")
	if err != nil {
		t.Fatalf("error creating realm: %v", err)
	}

	authApp1, err = db.CreateAuthorizedApp(realm.ID, "App 1", APIUserTypeAdmin)
	if err != nil {
		t.Fatalf("error creating app: %v", err)
	}
	authApp2, err = db.CreateAuthorizedApp(realm.ID, "App 2", APIUserTypeDevice)
	if err != nil {
		t.Fatalf("error creating app: %v", err)
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
