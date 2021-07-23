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

package database

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/go-cmp/cmp"
)

func TestUserStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats UserStats
		exp   string
	}{
		{
			name:  "empty",
			stats: nil,
			exp:   "",
		},
		{
			name: "single",
			stats: []*UserStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					UserID:      1,
					RealmID:     1,
					CodesIssued: 10,
					UserName:    "You",
					UserEmail:   "you@example.com",
				},
			},
			exp: `date,realm_id,user_id,user_name,user_email,codes_issued
2020-02-03,1,1,You,you@example.com,10
`,
		},
		{
			name: "multi",
			stats: []*UserStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					UserID:      1,
					RealmID:     1,
					CodesIssued: 10,
					UserName:    "You",
					UserEmail:   "you@example.com",
				},
				{
					Date:        time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					UserID:      2,
					RealmID:     1,
					CodesIssued: 45,
					UserName:    "Them",
					UserEmail:   "them@example.com",
				},
				{
					Date:        time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					UserID:      1,
					RealmID:     3,
					CodesIssued: 15,
					UserName:    "Us",
					UserEmail:   "us@example.com",
				},
			},
			exp: `date,realm_id,user_id,user_name,user_email,codes_issued
2020-02-03,1,1,You,you@example.com,10
2020-02-04,1,2,Them,them@example.com,45
2020-02-05,3,1,Us,us@example.com,15
`,
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
			if diff := cmp.Diff(string(b), tc.exp); diff != "" {
				t.Errorf("bad csv (+got, -want): %s", diff)
			}

			b, err = tc.stats.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			a := &UserStats{}
			err = a.UnmarshalJSON(b)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDatabase_PurgeUserStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	for i := 1; i < 10; i++ {
		ts := timeutils.UTCMidnight(time.Now().UTC()).Add(-24 * time.Hour * time.Duration(i))
		if err := db.RawDB().Create(&UserStat{
			Date: ts,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.PurgeUserStats(0); err != nil {
		t.Fatal(err)
	}

	var entries []*UserStat
	if err := db.RawDB().Model(&UserStat{}).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}

	if got, want := len(entries), 0; got != want {
		t.Errorf("expected %d entries, got %d: %#v", want, got, entries)
	}
}
