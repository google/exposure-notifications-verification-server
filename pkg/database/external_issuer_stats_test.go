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

func TestExternalIssuerStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   ExternalIssuerStats
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
			stats: []*ExternalIssuerStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					IssuerID:    "user:2",
					CodesIssued: 10,
				},
			},
			expCSV: `date,realm_id,issuer_id,codes_issued
2020-02-03,1,user:2,10
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-03T00:00:00Z","issuer_data":[{"issuer_id":"user:2","codes_issued":10}]}]}`,
		},
		{
			name: "multi",
			stats: []*ExternalIssuerStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					IssuerID:    "user:2",
					CodesIssued: 10,
				},
				{
					Date:        time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					IssuerID:    "user:2",
					CodesIssued: 45,
				},
				{
					Date:        time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					IssuerID:    "user:2",
					CodesIssued: 15,
				},
			},
			expCSV: `date,realm_id,issuer_id,codes_issued
2020-02-03,1,user:2,10
2020-02-04,1,user:2,45
2020-02-05,1,user:2,15
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-05T00:00:00Z","issuer_data":[{"issuer_id":"user:2","codes_issued":15}]},{"date":"2020-02-04T00:00:00Z","issuer_data":[{"issuer_id":"user:2","codes_issued":45}]},{"date":"2020-02-03T00:00:00Z","issuer_data":[{"issuer_id":"user:2","codes_issued":10}]}]}`,
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

func TestDatabase_PurgeExternalIssuerStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	for i := 1; i < 10; i++ {
		ts := timeutils.UTCMidnight(time.Now().UTC()).Add(-24 * time.Hour * time.Duration(i))
		if err := db.RawDB().Create(&ExternalIssuerStat{
			Date: ts,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.PurgeExternalIssuerStats(0); err != nil {
		t.Fatal(err)
	}

	var entries []*ExternalIssuerStat
	if err := db.RawDB().Model(&ExternalIssuerStat{}).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}

	if got, want := len(entries), 0; got != want {
		t.Errorf("expected %d entries, got %d: %#v", want, got, entries)
	}
}
