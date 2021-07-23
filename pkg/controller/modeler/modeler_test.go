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

package modeler

import (
	"testing"
	"time"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func testModeler(tb testing.TB) *Controller {
	tb.Helper()

	ctx := project.TestContext(tb)
	db, dbConfig := testDatabaseInstance.NewDatabase(tb, nil)

	config := config.Modeler{
		Database: *dbConfig,
		RateLimit: ratelimit.Config{
			Type:    "IN_MEMORY",
			HMACKey: []byte(""),
		},
		Cache: cache.Config{
			Type:    "IN_MEMORY",
			HMACKey: []byte(""),
		},
	}
	if err := envconfig.ProcessWith(ctx, &config, envconfig.MapLookuper(nil)); err != nil {
		tb.Fatal(err)
	}

	store, err := memorystore.New(&memorystore.Config{
		Tokens:   100,
		Interval: 24 * time.Hour,
	})
	if err != nil {
		tb.Fatal(err)
	}

	modeler := New(ctx, &config, db, store, nil)
	return modeler
}

func TestRebuildModel(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	modeler := testModeler(t)
	db := modeler.db

	// Create the realm.
	realm := database.NewRealmWithDefaults("Statsylvania")
	realm.AbusePreventionEnabled = true
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	var curr int64
	nextDate := func() time.Time {
		curr++
		return time.Unix(0, curr*int64(24*time.Hour))
	}

	// Create some initial statistics.
	{
		if err := db.RawDB().Exec("TRUNCATE realm_stats").Error; err != nil {
			t.Fatal(err)
		}

		line := []uint{50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStat{
					Date:        nextDate(),
					RealmID:     realm.ID,
					CodesIssued: y,
				}).
				Error; err != nil {
				t.Fatal(err)
			}
		}

		// Build the model.
		if err := modeler.rebuildModels(ctx); err != nil {
			t.Fatal(err)
		}

		// Get the realm so we can check the value.
		realm, err := db.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionLimit, uint(50); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Add some more statistics
	{
		if err := db.RawDB().Exec("TRUNCATE realm_stats").Error; err != nil {
			t.Fatal(err)
		}

		line := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStat{
					Date:        nextDate(),
					RealmID:     realm.ID,
					CodesIssued: y,
				}).
				Error; err != nil {
				t.Fatal(err)
			}
		}

		// Build the model.
		if err := modeler.rebuildModels(ctx); err != nil {
			t.Fatal(err)
		}

		// Get the realm so we can check the value.
		realm, err := db.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionLimit, uint(22); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Test
	{
		if err := db.RawDB().Exec("TRUNCATE realm_stats").Error; err != nil {
			t.Fatal(err)
		}

		line := []uint{1, 26, 61, 13, 19, 50, 9, 20, 91, 187, 39, 4, 2, 5, 1}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStat{
					Date:        nextDate(),
					RealmID:     realm.ID,
					CodesIssued: y,
				}).
				Error; err != nil {
				t.Fatal(err)
			}
		}

		// Build the model.
		if err := modeler.rebuildModels(ctx); err != nil {
			t.Fatal(err)
		}

		// Get the realm so we can check the value.
		realm, err := db.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionLimit, uint(28); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Ensure we hit the floor
	{
		if err := db.RawDB().Exec("TRUNCATE realm_stats").Error; err != nil {
			t.Fatal(err)
		}

		line := []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStat{
					Date:        nextDate(),
					RealmID:     realm.ID,
					CodesIssued: y,
				}).
				Error; err != nil {
				t.Fatal(err)
			}
		}

		// Build the model.
		if err := modeler.rebuildModels(ctx); err != nil {
			t.Fatal(err)
		}

		// Get the realm so we can check the value.
		realm, err := db.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionLimit, uint(10); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}

	// Ensure we hit the ceiling
	{
		if err := db.RawDB().Exec("TRUNCATE realm_stats").Error; err != nil {
			t.Fatal(err)
		}

		line := make([]uint, 24)
		for i := range line {
			line[i] = uint(10000 * i)
		}

		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStat{
					Date:        nextDate(),
					RealmID:     realm.ID,
					CodesIssued: y,
				}).
				Error; err != nil {
				t.Fatal(err)
			}
		}

		// Build the model.
		if err := modeler.rebuildModels(ctx); err != nil {
			t.Fatal(err)
		}

		// Get the realm so we can check the value.
		realm, err := db.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.AbusePreventionLimit, uint(20000); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}
