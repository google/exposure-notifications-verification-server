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

package appsync

import (
	"context"
	"testing"

	"github.com/google/exposure-notifications-verification-server/pkg/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestAppSync(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	config := &config.AppSyncConfig{}
	c, _ := New(config, db, nil)

	t.Run("name", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("test")
		realm.RegionCode = "US-WA"
		if err := db.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatalf("error saving realm: %v", err)
		}

		m := &database.MobileApp{
			RealmID: realm.ID,
			OS:      database.OSTypeAndroid,
			AppID:   "testAppId",
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
		}
		db.SaveMobileApp(m, database.SystemTest)

		resp := &clients.AppsResponse{
			Apps: []clients.App{
				{
					Region: "US-WA",
					IsEnx:  true,
					AndroidTarget: clients.AndroidTarget{
						Namespace:              "android_app",
						PackageName:            "testAppID-butDifferent",
						SHA256CertFingerprints: "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
					},
				}, {
					Region: "US-WA",
					IsEnx:  true,
					AndroidTarget: clients.AndroidTarget{
						Namespace:              "android_app",
						PackageName:            "testAppId2",
						SHA256CertFingerprints: "BB:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
					},
				},
			},
		}

		merr := c.syncApps(context.Background(), resp)
		if merr != nil {
			t.Fatalf(merr.ErrorOrNil().Error())
		}

		apps, err := db.ListActiveApps(realm.ID)
		if err != nil {
			t.Fatal("failed to list apps", err)
		}

		if len(apps) != 2 {
			t.Errorf("got %d apps, expected 2", len(apps))
		}
	})
}
