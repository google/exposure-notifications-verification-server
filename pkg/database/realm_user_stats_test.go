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

	"github.com/google/go-cmp/cmp"
)

func TestRealmUserStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   RealmUserStats
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
			stats: []*RealmUserStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					UserID:      1,
					Name:        "You",
					Email:       "you@example.com",
					CodesIssued: 10,
				},
			},
			expCSV: `date,realm_id,user_id,name,email,codes_issued
2020-02-03,1,1,You,you@example.com,10
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-03T00:00:00Z","issuer_data":[{"user_id":1,"name":"You","email":"you@example.com","codes_issued":10}]}]}`,
		},
		{
			name: "multi",
			stats: []*RealmUserStat{
				{
					Date:        time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					UserID:      1,
					Name:        "You",
					Email:       "you@example.com",
					CodesIssued: 10,
				},
				{
					Date:        time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					UserID:      2,
					Name:        "Them",
					Email:       "them@example.com",
					CodesIssued: 45,
				},
				{
					Date:        time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					RealmID:     1,
					UserID:      3,
					Name:        "Us",
					Email:       "us@example.com",
					CodesIssued: 15,
				},
			},
			expCSV: `date,realm_id,user_id,name,email,codes_issued
2020-02-03,1,1,You,you@example.com,10
2020-02-04,1,2,Them,them@example.com,45
2020-02-05,1,3,Us,us@example.com,15
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-05T00:00:00Z","issuer_data":[{"user_id":3,"name":"Us","email":"us@example.com","codes_issued":15}]},{"date":"2020-02-04T00:00:00Z","issuer_data":[{"user_id":2,"name":"Them","email":"them@example.com","codes_issued":45}]},{"date":"2020-02-03T00:00:00Z","issuer_data":[{"user_id":1,"name":"You","email":"you@example.com","codes_issued":10}]}]}`,
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
