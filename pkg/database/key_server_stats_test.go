// Copyright 2021 Google LLC
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
	"time"
)

func TestSaveKeyServerStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	err = db.SaveKeyServerStats(&KeyServerStats{
		RealmID:           realm.ID,
		KeyServerURL:      "TestKeyServerURL",
		KeyServerAudience: "TestAud",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetKeyServerStats(realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stats.KeyServerURL, "TestKeyServerURL"; got != want {
		t.Errorf("failed retrieving KeyServerStats. got %s, wanted %s", got, want)
	}
}

func TestSaveKeyServerStatsDay(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	err = db.SaveKeyServerStatsDay(&KeyServerStatsDay{
		RealmID:            realm.ID,
		Day:                now,
		TotalTEKsPublished: 50,
		TEKAgeDistribution: []int64{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetKeyServerStatsDay(realm.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stats.TotalTEKsPublished, int64(50); got != want {
		t.Errorf("failed retrieving KeyServerStats. got %d, wanted %d", got, want)
	}
	if got, want := stats.TEKAgeDistribution[4], int64(5); got != want {
		t.Errorf("failed retrieving KeyServerStats. got %d, wanted %d", got, want)
	}
}
