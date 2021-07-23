// Copyright 2020 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func TestMembership_AfterFind(t *testing.T) {
	t.Parallel()

	t.Run("user", func(t *testing.T) {
		t.Parallel()

		var m Membership
		_ = m.AfterFind()
		if errs := m.ErrorsFor("user"); len(errs) < 1 {
			t.Errorf("expected errors for %s", "user")
		}
	})

	t.Run("realm", func(t *testing.T) {
		t.Parallel()

		var m Membership
		_ = m.AfterFind()
		if errs := m.ErrorsFor("realm"); len(errs) < 1 {
			t.Errorf("expected errors for %s", "realm")
		}
	})
}

func TestMembership_SaveMembership(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm := NewRealmWithDefaults("test")
	if err := db.SaveRealm(realm, SystemTest); err != nil {
		t.Fatalf("error saving realm: %v", err)
	}

	user := &User{
		Email:       "user@example.com",
		Name:        "Dr User",
		SystemAdmin: true,
	}
	if err := db.SaveUser(user, System); err != nil {
		t.Fatalf("error creating user: %v", err)
	}

	if err := user.AddToRealm(db, realm, rbac.LegacyRealmAdmin, SystemTest); err != nil {
		t.Fatalf("failed adding user to realm %v", err)
	}

	m, err := user.FindMembership(db, realm.ID)
	if err != nil {
		t.Fatalf("failed finding membership %v", err)
	}

	m.DefaultSMSTemplateLabel = "This one"
	if err = db.SaveMembership(m, SystemTest); err != nil {
		t.Fatalf("failed saving membership %v", err)
	}

	m, err = user.FindMembership(db, realm.ID)
	if err != nil {
		t.Fatalf("failed finding membership %v", err)
	}

	if m.DefaultSMSTemplateLabel != "This one" {
		t.Fatalf("expected default template saved. got %s, want \"This one\"", m.DefaultSMSTemplateLabel)
	}

	found, err := user.SelectFirstMembership(db)
	if err != nil {
		t.Fatalf("failed finding membership %v", err)
	}

	if m.RealmID != found.RealmID {
		t.Fatalf("expected to find the same membership. got %v, want %v", m.RealmID, found.RealmID)
	}

	mList, err := user.ListMemberships(db)
	if err != nil {
		t.Fatalf("failed finding membership %v", err)
	}

	if len(mList) != 1 {
		t.Fatalf("membership ist length too short. got %d, want 1", len(mList))
	}

	if mList[0].RealmID != found.RealmID {
		t.Fatalf("expected to find the same membership. got %v, want %v", m.RealmID, found.RealmID)
	}
}
