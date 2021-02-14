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

package associated_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/associated"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestAndroidData(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.RedirectConfig{}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no_active_apps", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		c, err := associated.New(cfg, harness.Database, harness.Cacher, h)
		if err != nil {
			t.Fatal(err)
		}
		data, err := c.AndroidData(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(data), 0; got != want {
			t.Errorf("expected len(data) to be %d, got %d: %v", want, got, data)
		}
	})

	t.Run("active_apps", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		// Android app1
		app1 := &database.MobileApp{
			Name:    "app1",
			RealmID: realm.ID,
			URL:     "https://app1.example.com/",
			OS:      database.OSTypeAndroid,
			AppID:   "com.example.app1",
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
		}
		if err := harness.Database.SaveMobileApp(app1, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// Android app2
		app2 := &database.MobileApp{
			Name:    "app2",
			RealmID: realm.ID,
			URL:     "https://app2.example.com/",
			OS:      database.OSTypeAndroid,
			AppID:   "com.example.app2",
			SHA:     "BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB:BB",
		}
		if err := harness.Database.SaveMobileApp(app2, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// iOS app3, not Android
		app3 := &database.MobileApp{
			Name:    "app3",
			RealmID: realm.ID,
			URL:     "https://app3.example.com/",
			OS:      database.OSTypeIOS,
			AppID:   "com.example.app3",
		}
		if err := harness.Database.SaveMobileApp(app3, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// Android app4, not in realm
		app4 := &database.MobileApp{
			Name:    "app4",
			RealmID: realm.ID + 1, // Not this realm
			URL:     "https://app4.example.com/",
			OS:      database.OSTypeAndroid,
			AppID:   "com.example.app4",
			SHA:     "DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD:DD",
		}
		if err := harness.Database.SaveMobileApp(app4, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c, err := associated.New(cfg, harness.Database, harness.Cacher, h)
		if err != nil {
			t.Fatal(err)
		}
		data, err := c.AndroidData(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Ensure only the 2 actual apps that are android and of this realm were
		// included in the results.
		if got, want := len(data), 2; got != want {
			t.Errorf("expected len(data) to be %d, got %d: %v", want, got, data)
		}

		sort.Slice(data, func(i, j int) bool {
			return data[i].Target.PackageName < data[j].Target.PackageName
		})

		if got, want := data[0].Target.Fingerprints, []string{app1.SHA}; !reflect.DeepEqual(got, want) {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := data[1].Target.Fingerprints, []string{app2.SHA}; !reflect.DeepEqual(got, want) {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}
