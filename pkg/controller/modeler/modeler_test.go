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

package modeler

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter/memorystore"
)

func testModeler(tb testing.TB) *Controller {
	tb.Helper()

	ctx := context.Background()
	db, dbConfig := database.NewTestDatabaseWithConfig(tb)

	config := config.Modeler{
		Database: *dbConfig,
		RateLimit: ratelimit.Config{
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

	ctx := context.Background()
	modeler := testModeler(t)
	db := modeler.db

	// Create the realm.
	realm := database.NewRealmWithDefaults("Statsylvania")
	realm.AbusePreventionEnabled = true
	if err := db.SaveRealm(realm); err != nil {
		t.Fatal(err)
	}

	var curr int64
	nextDate := func() time.Time {
		curr++
		return time.Unix(0, curr*int64(24*time.Hour))
	}

	// Create some initial statistics.
	{
		line := []uint{50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStats{
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
		line := []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStats{
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

	// Ensure we hit the floor
	{
		line := []uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		for _, y := range line {
			if err := db.RawDB().
				Create(&database.RealmStats{
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
}
