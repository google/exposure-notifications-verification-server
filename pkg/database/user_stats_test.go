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
					Date:         time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					UserID:       1,
					RealmID:      1,
					CodesIssued:  10,
					CodesClaimed: 7,
					UserName:     "You",
					UserEmail:    "you@example.com",
				},
			},
			exp: `date,realm_id,user_id,user_name,user_email,codes_issued,codes_claimed
2020-02-03,1,1,You,you@example.com,10,7
`,
		},
		{
			name: "multi",
			stats: []*UserStat{
				{
					Date:         time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					UserID:       1,
					RealmID:      1,
					CodesIssued:  10,
					CodesClaimed: 7,
					UserName:     "You",
					UserEmail:    "you@example.com",
				},
				{
					Date:         time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					UserID:       2,
					RealmID:      1,
					CodesIssued:  45,
					CodesClaimed: 27,
					UserName:     "Them",
					UserEmail:    "them@example.com",
				},
				{
					Date:         time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					UserID:       1,
					RealmID:      3,
					CodesIssued:  15,
					CodesClaimed: 73,
					UserName:     "Us",
					UserEmail:    "us@example.com",
				},
			},
			exp: `date,realm_id,user_id,user_name,user_email,codes_issued,codes_claimed
2020-02-03,1,1,You,you@example.com,10,7
2020-02-04,1,2,Them,them@example.com,45,27
2020-02-05,3,1,Us,us@example.com,15,73
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
