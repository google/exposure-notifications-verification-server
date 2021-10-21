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

	"github.com/google/exposure-notifications-server/pkg/errcmp"
)

func TestRelamAdminPhone_Save(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		record  *RealmAdminPhone
		wantErr string
		errors  map[string]string
	}{
		{
			name: "",
			record: &RealmAdminPhone{
				RealmID:     realm.ID,
				Name:        "",
				PhoneNumber: "1",
			},
			wantErr: "validation failed",
			errors:  map[string]string{"name": "cannot be blank"},
		},
		{
			name: "",
			record: &RealmAdminPhone{
				RealmID:     realm.ID,
				Name:        "steve",
				PhoneNumber: "",
			},
			wantErr: "validation failed",
			errors:  map[string]string{"phoneNumber": "cannot be blank"},
		},
		{
			name: "",
			record: &RealmAdminPhone{
				RealmID:     realm.ID,
				Name:        "",
				PhoneNumber: "",
			},
			wantErr: "validation failed",
			errors: map[string]string{
				"name":        "cannot be blank",
				"phoneNumber": "cannot be blank",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := db.SaveRealmAdminPhone(tc.record, SystemTest)
			errcmp.MustMatch(t, err, tc.wantErr)

			for k, v := range tc.errors {
				got, ok := tc.record.errors[k]
				if !ok {
					t.Fatalf("no errors for key: %q", k)
				}
				found := false
				for _, e := range got {
					if e == v {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("missing expected error %q for key %q", v, k)
				}
			}
		})
	}
}
