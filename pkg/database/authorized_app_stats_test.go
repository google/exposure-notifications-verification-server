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
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/go-cmp/cmp"
)

func TestAuthorizedAppStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   AuthorizedAppStats
		expCSV  string
		expJSON string
	}{
		{
			name:    "empty",
			stats:   nil,
			expCSV:  ``,
			expJSON: `{}`,
		},
		{
			name: "single",
			stats: []*AuthorizedAppStat{
				{
					Date:              time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					AuthorizedAppID:   1,
					CodesIssued:       10,
					CodesClaimed:      4,
					CodesInvalid:      2,
					TokensClaimed:     3,
					TokensInvalid:     1,
					AuthorizedAppName: "Appy",
					AuthorizedAppType: "device",
				},
			},
			expCSV: `date,authorized_app_id,authorized_app_name,authorized_app_type,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid
2020-02-03,1,Appy,device,10,4,2,3,1
`,
			expJSON: `{"authorized_app_id":1,"authorized_app_name":"Appy","authorized_app_type":"device","statistics":[{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":4,"codes_invalid":2,"tokens_claimed":3,"tokens_invalid":1}}]}`,
		},
		{
			name: "multi",
			stats: []*AuthorizedAppStat{
				{
					Date:              time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					AuthorizedAppID:   1,
					CodesIssued:       10,
					CodesClaimed:      10,
					CodesInvalid:      2,
					TokensClaimed:     4,
					TokensInvalid:     2,
					AuthorizedAppName: "Appy",
					AuthorizedAppType: "device",
				},
				{
					Date:              time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					AuthorizedAppID:   1,
					CodesIssued:       45,
					CodesClaimed:      44,
					CodesInvalid:      5,
					TokensClaimed:     3,
					TokensInvalid:     2,
					AuthorizedAppName: "Mc",
					AuthorizedAppType: "admin",
				},
				{
					Date:              time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					AuthorizedAppID:   1,
					CodesIssued:       15,
					CodesClaimed:      13,
					CodesInvalid:      4,
					TokensClaimed:     6,
					TokensInvalid:     2,
					AuthorizedAppName: "Apperson",
					AuthorizedAppType: "stats",
				},
			},
			expCSV: `date,authorized_app_id,authorized_app_name,authorized_app_type,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid
2020-02-03,1,Appy,device,10,10,2,4,2
2020-02-04,1,Mc,admin,45,44,5,3,2
2020-02-05,1,Apperson,stats,15,13,4,6,2
`,
			expJSON: `{"authorized_app_id":1,"authorized_app_name":"Appy","authorized_app_type":"device","statistics":[{"date":"2020-02-05T00:00:00Z","data":{"codes_issued":15,"codes_claimed":13,"codes_invalid":4,"tokens_claimed":6,"tokens_invalid":2}},{"date":"2020-02-04T00:00:00Z","data":{"codes_issued":45,"codes_claimed":44,"codes_invalid":5,"tokens_claimed":3,"tokens_invalid":2}},{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":10,"codes_invalid":2,"tokens_claimed":4,"tokens_invalid":2}}]}`,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := tc.stats.MarshalCSV()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(b), tc.expCSV); diff != "" {
				t.Errorf("bad csv (+got, -want): %s", diff)
			}

			b, err = tc.stats.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(b), tc.expJSON; got != want {
				t.Errorf("bad json, expected \n%s\nto be\n%s\n", got, want)
			}
		})
	}
}

func TestDatabase_PurgeAuthorizedAppStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	for i := 1; i < 10; i++ {
		ts := timeutils.UTCMidnight(time.Now().UTC()).Add(-24 * time.Hour * time.Duration(i))
		if err := db.RawDB().Create(&AuthorizedAppStat{
			Date: ts,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.PurgeAuthorizedAppStats(0); err != nil {
		t.Fatal(err)
	}

	var entries []*AuthorizedAppStat
	if err := db.RawDB().Model(&AuthorizedAppStat{}).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}

	if got, want := len(entries), 0; got != want {
		t.Errorf("expected %d entries, got %d: %#v", want, got, entries)
	}
}
