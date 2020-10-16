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

import "testing"

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
