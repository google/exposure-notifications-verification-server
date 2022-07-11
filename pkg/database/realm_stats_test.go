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

func TestRealmStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   RealmStats
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
			stats: []*RealmStat{
				{
					Date:                     time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:                  1,
					CodesIssued:              10,
					CodesClaimed:             9,
					CodesInvalid:             1,
					CodesInvalidByOS:         []int64{0, 0, 0},
					TokensClaimed:            7,
					TokensInvalid:            2,
					CodeClaimMeanAge:         FromDuration(time.Minute),
					CodeClaimAgeDistribution: []int32{1, 3, 4},
				},
			},
			expCSV: `date,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid,code_claim_mean_age_seconds,code_claim_age_distribution,user_reports_issued,user_reports_claimed,user_report_tokens_claimed,codes_invalid_unknown_os,codes_invalid_ios,codes_invalid_android,user_reports_invalid_nonce
2020-02-03,10,9,1,7,2,60,1|3|4,0,0,0,0,0,0,0
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":9,"codes_invalid":1,"codes_invalid_by_os":{"unknown_os":0,"ios":0,"android":0},"user_reports_issued":0,"user_reports_claimed":0,"user_reports_invalid_nonce":0,"tokens_claimed":7,"tokens_invalid":2,"user_report_tokens_claimed":0,"code_claim_mean_age_seconds":60,"code_claim_age_distribution":[1,3,4]}}]}`,
		},
		{
			name: "multi",
			stats: []*RealmStat{
				{
					Date:                     time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmID:                  1,
					CodesIssued:              10,
					CodesClaimed:             9,
					CodesInvalid:             1,
					CodesInvalidByOS:         []int64{1, 2, 3},
					TokensClaimed:            7,
					TokensInvalid:            2,
					CodeClaimMeanAge:         FromDuration(time.Minute),
					CodeClaimAgeDistribution: []int32{1, 2, 3},
				},
				{
					Date:                     time.Date(2020, 2, 4, 0, 0, 0, 0, time.UTC),
					RealmID:                  1,
					CodesIssued:              45,
					CodesClaimed:             30,
					CodesInvalid:             29,
					CodesInvalidByOS:         []int64{0, 20, 9},
					TokensClaimed:            27,
					TokensInvalid:            2,
					CodeClaimMeanAge:         FromDuration(time.Hour),
					CodeClaimAgeDistribution: []int32{4, 5, 6},
				},
				{
					Date:                     time.Date(2020, 2, 5, 0, 0, 0, 0, time.UTC),
					RealmID:                  1,
					CodesIssued:              15,
					CodesClaimed:             2,
					UserReportsIssued:        2,
					UserReportsClaimed:       1,
					UserReportsInvalidNonce:  32,
					CodesInvalid:             0,
					CodesInvalidByOS:         []int64{0, 0, 0},
					TokensClaimed:            2,
					TokensInvalid:            0,
					UserReportTokensClaimed:  1,
					CodeClaimMeanAge:         FromDuration(time.Millisecond),
					CodeClaimAgeDistribution: []int32{7, 8, 9},
				},
			},
			expCSV: `date,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid,code_claim_mean_age_seconds,code_claim_age_distribution,user_reports_issued,user_reports_claimed,user_report_tokens_claimed,codes_invalid_unknown_os,codes_invalid_ios,codes_invalid_android,user_reports_invalid_nonce
2020-02-03,10,9,1,7,2,60,1|2|3,0,0,0,1,2,3,0
2020-02-04,45,30,29,27,2,3600,4|5|6,0,0,0,0,20,9,0
2020-02-05,15,2,0,2,0,0,7|8|9,2,1,1,0,0,0,32
`,
			expJSON: `{"realm_id":1,"statistics":[{"date":"2020-02-05T00:00:00Z","data":{"codes_issued":15,"codes_claimed":2,"codes_invalid":0,"codes_invalid_by_os":{"unknown_os":0,"ios":0,"android":0},"user_reports_issued":2,"user_reports_claimed":1,"user_reports_invalid_nonce":32,"tokens_claimed":2,"tokens_invalid":0,"user_report_tokens_claimed":1,"code_claim_mean_age_seconds":0,"code_claim_age_distribution":[7,8,9]}},{"date":"2020-02-04T00:00:00Z","data":{"codes_issued":45,"codes_claimed":30,"codes_invalid":29,"codes_invalid_by_os":{"unknown_os":0,"ios":20,"android":9},"user_reports_issued":0,"user_reports_claimed":0,"user_reports_invalid_nonce":0,"tokens_claimed":27,"tokens_invalid":2,"user_report_tokens_claimed":0,"code_claim_mean_age_seconds":3600,"code_claim_age_distribution":[4,5,6]}},{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":9,"codes_invalid":1,"codes_invalid_by_os":{"unknown_os":1,"ios":2,"android":3},"user_reports_issued":0,"user_reports_claimed":0,"user_reports_invalid_nonce":0,"tokens_claimed":7,"tokens_invalid":2,"user_report_tokens_claimed":0,"code_claim_mean_age_seconds":60,"code_claim_age_distribution":[1,2,3]}}]}`,
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

func TestDatabase_PurgeRealmStats(t *testing.T) {
	t.Parallel()

	db, _ := testDatabaseInstance.NewDatabase(t, nil)

	for i := 1; i < 10; i++ {
		ts := timeutils.UTCMidnight(time.Now().UTC()).Add(-24 * time.Hour * time.Duration(i))
		if err := db.RawDB().Create(&RealmStat{
			Date: ts,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := db.PurgeRealmStats(0); err != nil {
		t.Fatal(err)
	}

	var entries []*RealmStat
	if err := db.RawDB().Model(&RealmStat{}).Find(&entries).Error; err != nil {
		t.Fatal(err)
	}

	if got, want := len(entries), 0; got != want {
		t.Errorf("expected %d entries, got %d: %#v", want, got, entries)
	}
}
