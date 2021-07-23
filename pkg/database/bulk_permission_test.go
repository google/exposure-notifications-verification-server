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

package database

import (
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func TestBulkPermission_Apply(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		existingPerms rbac.Permission
		changePerms   rbac.Permission
		action        BulkPermissionAction
		expPerms      rbac.Permission
	}{
		// Add
		{
			name:        "adds_one",
			changePerms: rbac.CodeIssue,
			action:      BulkPermissionActionAdd,
			expPerms:    rbac.CodeIssue,
		},
		{
			name:        "adds_many",
			changePerms: rbac.CodeIssue | rbac.APIKeyRead,
			action:      BulkPermissionActionAdd,
			expPerms:    rbac.CodeIssue | rbac.APIKeyRead,
		},
		{
			name:          "adds_already_has",
			existingPerms: rbac.CodeIssue | rbac.APIKeyRead,
			changePerms:   rbac.CodeIssue | rbac.APIKeyRead,
			action:        BulkPermissionActionAdd,
			expPerms:      rbac.CodeIssue | rbac.APIKeyRead,
		},
		{
			name:          "adds_already_has_subset",
			existingPerms: rbac.CodeIssue,
			changePerms:   rbac.CodeIssue | rbac.APIKeyRead,
			action:        BulkPermissionActionAdd,
			expPerms:      rbac.CodeIssue | rbac.APIKeyRead,
		},

		// Remove
		{
			name:          "remove_one",
			existingPerms: rbac.CodeIssue,
			changePerms:   rbac.CodeIssue,
			action:        BulkPermissionActionRemove,
			expPerms:      0,
		},
		{
			name:          "remove_many",
			existingPerms: rbac.CodeIssue | rbac.APIKeyRead,
			changePerms:   rbac.CodeIssue | rbac.APIKeyRead,
			action:        BulkPermissionActionRemove,
			expPerms:      0,
		},
		{
			name:          "remove_doesnt_have",
			existingPerms: rbac.CodeIssue,
			changePerms:   rbac.APIKeyRead,
			action:        BulkPermissionActionRemove,
			expPerms:      rbac.CodeIssue,
		},
		{
			name:          "remove_doesnt_have_subset",
			existingPerms: rbac.CodeIssue | rbac.APIKeyRead,
			changePerms:   rbac.APIKeyRead,
			action:        BulkPermissionActionRemove,
			expPerms:      rbac.CodeIssue,
		},
		{
			name:          "remove_implied_added",
			existingPerms: rbac.APIKeyRead | rbac.APIKeyWrite,
			changePerms:   rbac.APIKeyRead,
			action:        BulkPermissionActionRemove,
			expPerms:      rbac.APIKeyRead | rbac.APIKeyWrite,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, _ := testDatabaseInstance.NewDatabase(t, nil)

			realm, err := db.FindRealm(1)
			if err != nil {
				t.Fatal(err)
			}

			// Create users in the realm with the provided permissions
			user1 := &User{
				Email: "one@example.com",
				Name:  "One",
			}
			if err := db.SaveUser(user1, SystemTest); err != nil {
				t.Fatal(err)
			}
			if err := user1.AddToRealm(db, realm, tc.existingPerms, SystemTest); err != nil {
				t.Fatal(err)
			}
			user2 := &User{
				Email: "two@example.com",
				Name:  "two",
			}
			if err := db.SaveUser(user2, SystemTest); err != nil {
				t.Fatal(err)
			}
			if err := user2.AddToRealm(db, realm, tc.existingPerms, SystemTest); err != nil {
				t.Fatal(err)
			}

			// Perform bulk operation
			bulk := &BulkPermission{
				RealmID:     realm.ID,
				UserIDs:     []uint{user1.ID, user2.ID},
				Permissions: tc.changePerms,
				Action:      tc.action,
			}
			if err := bulk.Apply(db, user1); err != nil {
				t.Fatal(err)
			}

			// Can't modify self, so these should be unchanged from given
			membership1, err := user1.FindMembership(db, realm.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := membership1.Permissions, tc.existingPerms; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}

			membership2, err := user2.FindMembership(db, realm.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := membership2.Permissions, tc.expPerms; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
	}
}
