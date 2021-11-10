// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"reflect"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

const thirtyDays = 30 * 24 * time.Hour

func TestSaveKeyServerStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	err = db.SaveKeyServerStats(&KeyServerStats{
		RealmID:                   realm.ID,
		KeyServerURLOverride:      "TestKeyServerURL",
		KeyServerAudienceOverride: "TestAud",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetKeyServerStats(realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stats.KeyServerURLOverride, "TestKeyServerURL"; got != want {
		t.Errorf("failed retrieving KeyServerStats. got %s, wanted %s", got, want)
	}

	list, err := db.ListKeyServerStats()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := list[0].RealmID, stats.RealmID; got != want {
		t.Errorf("failed listing the stats configs. got realm %d, wanted realm %d", got, want)
	}

	err = db.DeleteKeyServerStats(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.GetKeyServerStats(realm.ID)
	if err != nil && !IsNotFound(err) {
		t.Fatal(err)
	}
}

func TestSaveKeyServerStatsDay(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)
	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	now := timeutils.UTCMidnight(time.Now())

	err = db.SaveKeyServerStatsDay(&KeyServerStatsDay{
		RealmID:            realm.ID,
		Day:                now,
		TotalTEKsPublished: 50,
		TEKAgeDistribution: []int64{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := db.ListKeyServerStatsDays(realm.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := stats[0].TotalTEKsPublished, int64(50); got != want {
		t.Errorf("failed retrieving KeyServerStats. got %d, wanted %d", got, want)
	}
	if got, want := stats[0].TEKAgeDistribution[4], int64(5); got != want {
		t.Errorf("failed retrieving KeyServerStats. got %d, wanted %d", got, want)
	}

	err = db.SaveKeyServerStatsDay(&KeyServerStatsDay{
		RealmID:            realm.ID,
		Day:                now.Add(-50 * 24 * time.Hour),
		TotalTEKsPublished: 50,
		TEKAgeDistribution: []int64{1, 2, 3, 4, 5},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err = db.ListKeyServerStatsDays(realm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(stats), project.StatsDisplayDays+1; got != want {
		t.Errorf("incorrect number of stats. got %d, want %d", got, want)
	}

	rows, err := db.DeleteOldKeyServerStatsDays(thirtyDays)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("expected purged row, got %d", rows)
	}
}

func TestConvertStatsDay(t *testing.T) {
	t.Parallel()

	now := time.Now()
	day := &KeyServerStatsDay{
		RealmID:            1,
		Day:                now,
		TotalTEKsPublished: 50,
		PublishRequests:    []int64{1, 2, 3},
		TEKAgeDistribution: []int64{1, 2, 3, 4, 5},
	}

	resp := day.ToResponse()
	roundTripped := MakeKeyServerStatsDay(1, resp)

	if !reflect.DeepEqual(day, roundTripped) {
		t.Errorf("round trip failed. got %#v want %#v", roundTripped, day)
	}
}
