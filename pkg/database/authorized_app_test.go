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
)

func TestDatabase_CreateFindAPIKey(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("foo")
	if err != nil {
		t.Fatal(err)
	}

	authApp := &AuthorizedApp{
		Name:       "University System Health Org",
		APIKeyType: APIUserTypeAdmin,
	}

	apiKey, err := realm.CreateAuthorizedApp(db, authApp)
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

	db := NewTestDatabase(t)

	realm, err := db.CreateRealm("bar")
	if err != nil {
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

	db := NewTestDatabase(t)

	apiKey, realmID := "abcd1234", uint(15)

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
