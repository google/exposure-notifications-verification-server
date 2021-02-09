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

func TestRealmUserStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats RealmUserStats
		exp   string
	}{
		{
			name:  "empty",
			stats: nil,
			exp:   "",
		},
		{
			name: "single",
			stats: []*RealmUserStat{
				{
					Date:         time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:      1,
					UserID:       1,
					Name:         "You",
					Email:        "you@example.com",
					CodesIssued:  10,
					CodesClaimed: 7,
				},
			},
			exp: `date,realm_id,user_id,name,email,codes_issued,codes_claimed
2020-02-03,1,1,You,you@example.com,10,7
`,
		},
		{
			name: "multi",
			stats: []*RealmUserStat{
				{
					Date:         time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:      1,
					UserID:       1,
					Name:         "You",
					Email:        "you@example.com",
					CodesIssued:  10,
					CodesClaimed: 7,
				},
				{
					Date:         time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					RealmID:      1,
					UserID:       2,
					Name:         "Them",
					Email:        "them@example.com",
					CodesIssued:  45,
					CodesClaimed: 27,
				},
				{
					Date:         time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					RealmID:      1,
					UserID:       3,
					Name:         "Us",
					Email:        "us@example.com",
					CodesIssued:  15,
					CodesClaimed: 73,
				},
			},
			exp: `date,realm_id,user_id,name,email,codes_issued,codes_claimed
2020-02-03,1,1,You,you@example.com,10,7
2020-02-04,1,2,Them,them@example.com,45,27
2020-02-05,1,3,Us,us@example.com,15,73
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
			a := &RealmUserStats{}
			err = a.UnmarshalJSON(b)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
