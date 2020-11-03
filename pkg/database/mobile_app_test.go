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

	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
)

func TestMobileApp_Validation(t *testing.T) {
	t.Parallel()

	db := NewTestDatabase(t)

	t.Run("name", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.Name = ""
		_ = m.BeforeSave(db.RawDB())

		nameErrs := m.ErrorsFor("name")
		if len(nameErrs) < 1 {
			t.Fatal("expected error")
		}
	})

	t.Run("app_id", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.AppID = ""
		_ = m.BeforeSave(db.RawDB())

		appIDErrs := m.ErrorsFor("app_id")
		if len(appIDErrs) < 1 {
			t.Fatal("expected error")
		}
	})

	t.Run("os", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.OS = 0
		_ = m.BeforeSave(db.RawDB())

		osErrs := m.ErrorsFor("os")
		if len(osErrs) < 1 {
			t.Fatal("expected error")
		}

		m.OS = 4
		_ = m.BeforeSave(db.RawDB())

		osErrs = m.ErrorsFor("os")
		if len(osErrs) < 1 {
			t.Fatal("expected error")
		}
	})

	t.Run("url", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.URL = ""
		_ = m.BeforeSave(db.RawDB())

		urlErrors := m.ErrorsFor("url")
		if len(urlErrors) != 0 {
			t.Fatal("no xpected error")
		}
	})

	t.Run("sha", func(t *testing.T) {
		t.Parallel()

		var m MobileApp
		m.OS = OSTypeIOS
		_ = m.BeforeSave(db.RawDB())

		shaErrs := m.ErrorsFor("sha")
		if len(shaErrs) > 0 {
			t.Fatalf("expected no errors: %v", shaErrs)
		}

		m.OS = OSTypeAndroid
		_ = m.BeforeSave(db.RawDB())

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
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var m MobileApp
				m.SHA = tc.sha
				_ = m.BeforeSave(db.RawDB())

				shaErrs := m.ErrorsFor("sha")
				if !tc.err && len(shaErrs) > 0 {
					t.Errorf("validation failed: %v", shaErrs)
				}
			})
		}
	})
}

func TestMobileApp_List(t *testing.T) {
	t.Parallel()

	t.Run("access_mobileapps_and_realms", func(t *testing.T) {
		db := NewTestDatabase(t)

		realm1 := NewRealmWithDefaults("realm1")
		if err := db.SaveRealm(realm1, System); err != nil {
			t.Fatal(err)
		}

		realm2 := NewRealmWithDefaults("realm2")
		if err := db.SaveRealm(realm2, System); err != nil {
			t.Fatal(err)
		}

		app1 := &MobileApp{
			Name:    "app1",
			RealmID: realm1.ID,
			URL:     "https://example1.com",
			OS:      OSTypeIOS,
			AppID:   "app1",
		}
		if err := db.SaveMobileApp(app1, System); err != nil {
			t.Fatal(err)
		}

		app2 := &MobileApp{
			Name:    "app2",
			RealmID: realm1.ID,
			URL:     "https://example2.com",
			OS:      OSTypeAndroid,
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
			AppID:   "app2",
		}
		if err := db.SaveMobileApp(app2, System); err != nil {
			t.Fatal(err)
		}

		page := &pagination.PageParams{
			Page:  0,
			Limit: pagination.DefaultLimit,
		}
		extapp, _, err := db.ListActiveAppsWithRealm(page)
		if err != nil {
			t.Fatal(err)
		}

		if len(extapp) != 2 {
			t.Errorf("got %v apps, wanted: 2", len(extapp))
		}

		apps, err := db.ListActiveAppsByOS(realm1.ID, OSTypeAndroid)
		if err != nil {
			t.Fatal(err)
		}

		if len(apps) != 1 {
			t.Errorf("got %v apps, wanted: 1", len(apps))
		}
	})
}
