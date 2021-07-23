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

package associated_test

import (
	"sort"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/associated"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestIOSData(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.RedirectConfig{}

	h, err := render.New(ctx, nil, true)
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
		data, err := c.IOSData(realm.ID)
		if err != nil {
			t.Fatal(err)
		}
		if data != nil {
			t.Errorf("expected data to be nil, got %#v", data)
		}
	})

	t.Run("active_apps", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		// iOS app1
		app1 := &database.MobileApp{
			Name:    "app1",
			RealmID: realm.ID,
			URL:     "https://app1.example.com/",
			OS:      database.OSTypeIOS,
			AppID:   "com.example.app1",
		}
		if err := harness.Database.SaveMobileApp(app1, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// iOS app2
		app2 := &database.MobileApp{
			Name:    "app2",
			RealmID: realm.ID,
			URL:     "https://app2.example.com/",
			OS:      database.OSTypeIOS,
			AppID:   "com.example.app2",
		}
		if err := harness.Database.SaveMobileApp(app2, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// Android app3, not IOS
		app3 := &database.MobileApp{
			Name:    "app3",
			RealmID: realm.ID,
			URL:     "https://app3.example.com/",
			OS:      database.OSTypeAndroid,
			AppID:   "com.example.app3",
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
		}
		if err := harness.Database.SaveMobileApp(app3, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		// iOS app4, not in realm
		app4 := &database.MobileApp{
			Name:    "app4",
			RealmID: realm.ID + 1, // Not this realm
			URL:     "https://app4.example.com/",
			OS:      database.OSTypeIOS,
			AppID:   "com.example.app4",
		}
		if err := harness.Database.SaveMobileApp(app4, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		c, err := associated.New(cfg, harness.Database, harness.Cacher, h)
		if err != nil {
			t.Fatal(err)
		}
		data, err := c.IOSData(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		// Ensure only the 2 actual apps that are ios and of this realm were
		// included in the results.
		details := data.Applinks.Details
		if got, want := len(details), 2; got != want {
			t.Errorf("expected len(details) to be %d, got %d: %v", want, got, details)
		}

		sort.Slice(details, func(i, j int) bool {
			return details[i].AppID < details[j].AppID
		})

		if got, want := details[0].AppID, app1.AppID; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := details[1].AppID, app2.AppID; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		// Apps should exist, but be empty (Apple requirement)
		if data.Applinks.Apps == nil {
			t.Fatalf("Applinks.Apps should not be nil")
		}
		if len(data.Applinks.Apps) != 0 {
			t.Errorf("AppLinks.Apps should be empty: %v", data.Applinks.Apps)
		}
	})
}
