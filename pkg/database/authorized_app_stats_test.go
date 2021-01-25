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

func TestAuthorizedAppStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stats AuthorizedAppStats
		exp   string
	}{
		{
			name:  "empty",
			stats: nil,
			exp:   "",
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
			exp: `date,authorized_app_id,authorized_app_name,authorized_app_type,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid
2020-02-03,1,Appy,device,10,4,2,3,1
`,
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
			exp: `date,authorized_app_id,authorized_app_name,authorized_app_type,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid
2020-02-03,1,Appy,device,10,10,2,4,2
2020-02-04,1,Mc,admin,45,44,5,3,2
2020-02-05,1,Apperson,stats,15,13,4,6,2
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
			a := &AuthorizedAppStats{}
			err = a.UnmarshalJSON(b)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
