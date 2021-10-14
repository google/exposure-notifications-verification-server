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
	"math"
	"testing"
	"time"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter/memorystore"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
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

func TestRebuildAbusePreventionModel(t *testing.T) {
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
		if err := modeler.rebuildAbusePreventionModel(ctx, realm); err != nil {
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
		if err := modeler.rebuildAbusePreventionModel(ctx, realm); err != nil {
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
		if err := modeler.rebuildAbusePreventionModel(ctx, realm); err != nil {
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
		if err := modeler.rebuildAbusePreventionModel(ctx, realm); err != nil {
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
		if err := modeler.rebuildAbusePreventionModel(ctx, realm); err != nil {
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

func TestRebuildAnomaliesModel(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cases := []struct {
		name  string
		stats []*database.RealmStat
		exp   [6]float64 // codes_claimed,mean,stddev,tokens_claimed,mean,stddev
	}{
		{
			name:  "zeros",
			stats: make([]*database.RealmStat, 16),
			exp:   [6]float64{0, 0, 0},
		},
		{
			name: "simple_mean_stddev",
			stats: []*database.RealmStat{
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
				{CodesIssued: 1, CodesClaimed: 1},
			},
			exp: [6]float64{1, 1, 0},
		},
		{
			name: "growing",
			stats: []*database.RealmStat{
				{CodesIssued: 5, CodesClaimed: 4},   // current date
				{CodesIssued: 29, CodesClaimed: 27}, // last whole date
				{CodesIssued: 28, CodesClaimed: 25},
				{CodesIssued: 27, CodesClaimed: 25},
				{CodesIssued: 26, CodesClaimed: 24},
				{CodesIssued: 25, CodesClaimed: 24},
				{CodesIssued: 24, CodesClaimed: 24},
				{CodesIssued: 23, CodesClaimed: 18},
				{CodesIssued: 22, CodesClaimed: 18},
				{CodesIssued: 21, CodesClaimed: 18},
				{CodesIssued: 20, CodesClaimed: 15},
				{CodesIssued: 19, CodesClaimed: 15},
				{CodesIssued: 18, CodesClaimed: 15},
				{CodesIssued: 17, CodesClaimed: 15},
				{CodesIssued: 16, CodesClaimed: 15},
				{CodesIssued: 15, CodesClaimed: 15},
			},
			exp: [6]float64{0.931034, 0.882318, 0.077307},
		},
		{
			name: "declining",
			stats: []*database.RealmStat{
				{CodesIssued: 1, CodesClaimed: 0}, // current date
				{CodesIssued: 2, CodesClaimed: 1}, // last whole date
				{CodesIssued: 4, CodesClaimed: 2},
				{CodesIssued: 8, CodesClaimed: 4},
				{CodesIssued: 8, CodesClaimed: 8},
				{CodesIssued: 9, CodesClaimed: 8},
				{CodesIssued: 10, CodesClaimed: 9},
				{CodesIssued: 14, CodesClaimed: 10},
				{CodesIssued: 16, CodesClaimed: 14},
				{CodesIssued: 18, CodesClaimed: 16},
				{CodesIssued: 22, CodesClaimed: 18},
				{CodesIssued: 38, CodesClaimed: 22},
				{CodesIssued: 54, CodesClaimed: 38},
				{CodesIssued: 55, CodesClaimed: 54},
				{CodesIssued: 56, CodesClaimed: 55},
				{CodesIssued: 58, CodesClaimed: 56},
			},
			exp: [6]float64{0.5, 0.806955, 0.171171},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Now().UTC()

			c := testModeler(t)
			// Create the realm.
			realm, err := c.db.FindRealm(1)
			if err != nil {
				t.Fatal(err)
			}

			// Create the stats.
			for i, v := range tc.stats {
				if v == nil {
					v = &database.RealmStat{}
				}
				v.RealmID = realm.ID
				v.Date = timeutils.UTCMidnight(now.Add(time.Duration(-i) * 24 * time.Hour))
				if err := c.db.RawDB().Create(v).Error; err != nil {
					t.Fatal(err)
				}
			}

			if err := c.rebuildAnomaliesModel(ctx, realm); err != nil {
				t.Fatal(err)
			}

			// Lookup the realm again to ensure it has updated values.
			realm, err = c.db.FindRealm(realm.ID)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := realm.LastCodesClaimedRatio, tc.exp[0]; !floatsEqual(got, want) {
				t.Errorf("expected %f to be %f", got, want)
			}
			if got, want := realm.CodesClaimedRatioMean, tc.exp[1]; !floatsEqual(got, want) {
				t.Errorf("expected %f to be %f", got, want)
			}
			if got, want := realm.CodesClaimedRatioStddev, tc.exp[2]; !floatsEqual(got, want) {
				t.Errorf("expected %f to be %f", got, want)
			}
		})
	}
}

func floatsEqual(a, b float64) bool {
	if diff, tolerance := math.Abs(a-b), 0.0001; diff < tolerance {
		return true
	}
	return false
}
