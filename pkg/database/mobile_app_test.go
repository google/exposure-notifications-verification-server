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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

func TestOSType(t *testing.T) {
	t.Parallel()

	// This test might seem like it's redundant, but it's designed to ensure that
	// the exact values for existing types remain unchanged.
	cases := []struct {
		t   OSType
		exp int
	}{
		{OSTypeUnknown, 0},
		{OSTypeIOS, 1},
		{OSTypeAndroid, 2},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.t.Display(), func(t *testing.T) {
			t.Parallel()

			if got, want := int(tc.t), tc.exp; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestOSType_Display(t *testing.T) {
	t.Parallel()

	cases := []struct {
		t   OSType
		exp string
	}{
		{OSTypeUnknown, "Unknown"},
		{OSTypeIOS, "iOS"},
		{OSTypeAndroid, "Android"},
		{1991, "Unknown"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(fmt.Sprintf("%d", tc.t), func(t *testing.T) {
			t.Parallel()

			if got, want := tc.t.Display(), tc.exp; got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		})
	}
}

func TestMobileApp_Validation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		structField string
		field       string
	}{
		{"Name", "name"},
		{"AppID", "app_id"},
		{"OS", "os"},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			exerciseValidation(t, &MobileApp{}, tc.structField, tc.field)
		})
	}

	t.Run("sha", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.OS = OSTypeIOS
		_ = m.BeforeSave(&gorm.DB{})

		shaErrs := m.ErrorsFor("sha")
		if len(shaErrs) > 0 {
			t.Fatalf("expected no errors: %v", shaErrs)
		}

		m.OS = OSTypeAndroid
		_ = m.BeforeSave(&gorm.DB{})

		shaErrs = m.ErrorsFor("sha")
		if len(shaErrs) < 1 {
			t.Fatal("expected error")
		}
	})

	t.Run("sha", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			sha  string
			err  bool
		}{
			{
				name: "empty",
				sha:  "",
				err:  true,
			},
			{
				name: "short",
				sha:  "abc",
				err:  true,
			},
			{
				name: "good",
				sha:  "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
			},
		}

		for _, tc := range cases {
			tc := tc

			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var m MobileApp
				m.SHA = tc.sha
				_ = m.BeforeSave(&gorm.DB{})

				shaErrs := m.ErrorsFor("sha")
				if !tc.err && len(shaErrs) > 0 {
					t.Errorf("validation failed: %v", shaErrs)
				}
			})
		}
	})
}

func TestDatabase_FindMobileApp(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	app := &MobileApp{
		Name:  "app1",
		Realm: realm,
		URL:   "https://example1.com",
		OS:    OSTypeIOS,
		AppID: "app1",
	}
	if err := db.SaveMobileApp(app, SystemTest); err != nil {
		t.Fatal(err, app.ErrorMessages())
	}

	cases := []struct {
		name string
		id   interface{}
		err  string
	}{
		{
			name: "not_found",
			id:   "123456",
			err:  "not found",
		},
		{
			name: "invalid_id",
			id:   "banana",
			err:  "invalid input syntax",
		},
		{
			name: "found",
			id:   app.ID,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found, err := db.FindMobileApp(tc.id)
			if err != nil {
				if tc.err != "" {
					if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
						t.Fatalf("expected %q to contain %q", got, want)
					}
				} else {
					t.Fatal(err)
				}
			}

			if found != nil {
				if got, want := found.ID, tc.id; got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
				if got, want := found.RealmID, realm.ID; got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
			}
		})
	}
}

func TestDatabase_PurgeMobileApps(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := db.SaveMobileApp(&MobileApp{
			RealmID: 1,
			Name:    fmt.Sprintf("Appy%d", i),
			OS:      OSTypeIOS,
			URL:     fmt.Sprintf("https://%d.example.com", i),
			AppID:   fmt.Sprintf("app.%d.com", i),
			Model: gorm.Model{
				DeletedAt: &now,
			},
		}, SystemTest); err != nil {
			t.Fatal(err)
		}
	}

	// Should not purge entries (too young).
	{
		n, err := db.PurgeMobileApps(24 * time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(0); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}

	// Purges entries.
	{
		n, err := db.PurgeMobileApps(1 * time.Nanosecond)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := n, int64(5); got != want {
			t.Errorf("expected %d to purge, got %d", want, got)
		}
	}
}

func TestMobileApp_Audits(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	realm1 := NewRealmWithDefaults("realm1")
	if err := db.SaveRealm(realm1, SystemTest); err != nil {
		t.Fatal(err)
	}

	app1 := &MobileApp{
		Name:    "app1",
		RealmID: realm1.ID,
		URL:     "https://example1.com",
		OS:      OSTypeIOS,
		AppID:   "app1",
	}
	if err := db.SaveMobileApp(app1, SystemTest); err != nil {
		t.Fatal(err)
	}

	app1.Name = "New Name"
	app1.URL = "https://new.url"
	app1.OS = OSTypeAndroid
	app1.AppID = "appNew"
	app1.SHA = "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA"
	if err := db.SaveMobileApp(app1, SystemTest); err != nil {
		t.Fatalf("%v, %v", err, app1.errors)
	}

	audits, _, err := db.ListAudits(&pagination.PageParams{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(audits), 6; got != want {
		t.Errorf("expected %d audits, got %d: %v", want, got, audits)
	}
}
