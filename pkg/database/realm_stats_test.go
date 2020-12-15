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

	"github.com/google/go-cmp/cmp"
)

func TestRealmStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats RealmStats
		exp   string
	}{
		{
			name:  "empty",
			stats: nil,
			exp:   "",
		},
		{
			name: "single",
			stats: []*RealmStat{
				{
					Date:             time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:          1,
					CodesIssued:      10,
					CodesClaimed:     9,
					DailyActiveUsers: 2,
				},
			},
			exp: `date,codes_issued,codes_claimed,daily_active_users
2020-02-03,10,9,2
`,
		},
		{
			name: "multi",
			stats: []*RealmStat{
				{
					Date:             time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:          1,
					CodesIssued:      10,
					CodesClaimed:     9,
					DailyActiveUsers: 12,
				},
				{
					Date:             time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					RealmID:          1,
					CodesIssued:      45,
					CodesClaimed:     30,
					DailyActiveUsers: 24,
				},
				{
					Date:             time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					RealmID:          1,
					CodesIssued:      15,
					CodesClaimed:     2,
					DailyActiveUsers: 18,
				},
			},
			exp: `date,codes_issued,codes_claimed,daily_active_users
2020-02-03,10,9,12
2020-02-04,45,30,24
2020-02-05,15,2,18
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
		})
	}
}
