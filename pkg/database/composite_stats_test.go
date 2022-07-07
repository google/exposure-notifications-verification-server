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
	"testing"
	"time"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/go-cmp/cmp"
)

func TestCompositeStats_MarshalCSV(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   CompositeStats
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
			stats: []*CompositeDay{
				{
					Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmStats: &RealmStat{
						Date:                     time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
						RealmID:                  1,
						CodesIssued:              10,
						CodesClaimed:             9,
						CodesInvalid:             1,
						CodesInvalidByOS:         []int64{0, 1, 0},
						UserReportsIssued:        3,
						UserReportsClaimed:       2,
						UserReportsInvalidNonce:  0,
						TokensClaimed:            7,
						TokensInvalid:            2,
						UserReportTokensClaimed:  2,
						CodeClaimMeanAge:         FromDuration(time.Minute),
						CodeClaimAgeDistribution: []int32{1, 3, 4},
					},
					KeyServerStats: &keyserver.StatsDay{
						Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
						PublishRequests: keyserver.PublishRequests{
							UnknownPlatform: 2,
							Android:         39,
							IOS:             12,
						},
						TotalTEKsPublished:        49,
						RevisionRequests:          3,
						TEKAgeDistribution:        []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
						OnsetToUploadDistribution: nil,
						RequestsMissingOnsetDate:  2,
					},
				},
			},
			expCSV: `date,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid,code_claim_mean_age_seconds,code_claim_age_distribution,publish_requests_unknown,publish_requests_android,publish_requests_ios,total_teks_published,requests_with_revisions,requests_missing_onset_date,tek_age_distribution,onset_to_upload_distribution,user_reports_issued,user_reports_claimed,user_report_tokens_claimed,codes_invalid_unknown_os,codes_invalid_ios,codes_invalid_android,user_reports_invalid_nonce
2020-02-03,10,9,1,7,2,60,1|3|4,2,39,12,49,3,2,0|1|2|3|4|5|6|7|8|9|10|11|12|13|14,,3,2,2,0,1,0,0
`,
			expJSON: `{"realm_id":1,"has_key_server_stats":true,"statistics":[{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":9,"codes_invalid":1,"codes_invalid_by_os":{"unknown_os":0,"ios":1,"android":0},"user_reports_issued":3,"user_reports_claimed":2,"user_reports_invalid_nonce":0,"tokens_claimed":7,"tokens_invalid":2,"user_report_tokens_claimed":2,"code_claim_mean_age_seconds":60,"code_claim_age_distribution":[1,3,4],"day":"0001-01-01T00:00:00Z","publish_requests":{"unknown":2,"android":39,"ios":12},"total_teks_published":49,"requests_with_revisions":3,"tek_age_distribution":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14],"onset_to_upload_distribution":null,"requests_missing_onset_date":2,"total_publish_requests":53}}]}`,
		},
		{
			name: "no_realm_stats",
			stats: []*CompositeDay{
				{
					Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					KeyServerStats: &keyserver.StatsDay{
						Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
						PublishRequests: keyserver.PublishRequests{
							UnknownPlatform: 2,
							Android:         39,
							IOS:             12,
						},
						TotalTEKsPublished:        49,
						RevisionRequests:          3,
						TEKAgeDistribution:        []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
						OnsetToUploadDistribution: nil,
						RequestsMissingOnsetDate:  2,
					},
				},
			},
			expCSV: `date,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid,code_claim_mean_age_seconds,code_claim_age_distribution,publish_requests_unknown,publish_requests_android,publish_requests_ios,total_teks_published,requests_with_revisions,requests_missing_onset_date,tek_age_distribution,onset_to_upload_distribution,user_reports_issued,user_reports_claimed,user_report_tokens_claimed,codes_invalid_unknown_os,codes_invalid_ios,codes_invalid_android,user_reports_invalid_nonce
2020-02-03,,,,,,,,2,39,12,49,3,2,0|1|2|3|4|5|6|7|8|9|10|11|12|13|14,,,,,,,,
`,
			expJSON: `{"realm_id":0,"has_key_server_stats":true,"statistics":[{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":0,"codes_claimed":0,"codes_invalid":0,"codes_invalid_by_os":{"unknown_os":0,"ios":0,"android":0},"user_reports_issued":0,"user_reports_claimed":0,"user_reports_invalid_nonce":0,"tokens_claimed":0,"tokens_invalid":0,"user_report_tokens_claimed":0,"code_claim_mean_age_seconds":0,"code_claim_age_distribution":null,"day":"0001-01-01T00:00:00Z","publish_requests":{"unknown":2,"android":39,"ios":12},"total_teks_published":49,"requests_with_revisions":3,"tek_age_distribution":[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14],"onset_to_upload_distribution":null,"requests_missing_onset_date":2,"total_publish_requests":53}}]}`,
		},
		{
			name: "no_keyserver_stats",
			stats: []*CompositeDay{
				{
					Day: time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
					RealmStats: &RealmStat{
						Date:                     time.Date(2020, 2, 3, 0, 0, 0, 0, time.UTC),
						RealmID:                  1,
						CodesIssued:              10,
						CodesClaimed:             9,
						CodesInvalid:             1,
						CodesInvalidByOS:         []int64{0, 1, 0},
						UserReportsIssued:        3,
						UserReportsClaimed:       2,
						UserReportsInvalidNonce:  1,
						TokensClaimed:            7,
						TokensInvalid:            2,
						UserReportTokensClaimed:  2,
						CodeClaimMeanAge:         FromDuration(time.Minute),
						CodeClaimAgeDistribution: []int32{1, 3, 4},
					},
				},
			},
			expCSV: `date,codes_issued,codes_claimed,codes_invalid,tokens_claimed,tokens_invalid,code_claim_mean_age_seconds,code_claim_age_distribution,publish_requests_unknown,publish_requests_android,publish_requests_ios,total_teks_published,requests_with_revisions,requests_missing_onset_date,tek_age_distribution,onset_to_upload_distribution,user_reports_issued,user_reports_claimed,user_report_tokens_claimed,codes_invalid_unknown_os,codes_invalid_ios,codes_invalid_android,user_reports_invalid_nonce
2020-02-03,10,9,1,7,2,60,1|3|4,,,,,,,,,3,2,2,0,1,0,1
`,
			expJSON: `{"realm_id":1,"has_key_server_stats":false,"statistics":[{"date":"2020-02-03T00:00:00Z","data":{"codes_issued":10,"codes_claimed":9,"codes_invalid":1,"codes_invalid_by_os":{"unknown_os":0,"ios":1,"android":0},"user_reports_issued":3,"user_reports_claimed":2,"user_reports_invalid_nonce":1,"tokens_claimed":7,"tokens_invalid":2,"user_report_tokens_claimed":2,"code_claim_mean_age_seconds":60,"code_claim_age_distribution":[1,3,4],"day":"0001-01-01T00:00:00Z","publish_requests":{"unknown":0,"android":0,"ios":0},"total_teks_published":0,"requests_with_revisions":0,"tek_age_distribution":null,"onset_to_upload_distribution":null,"requests_missing_onset_date":0,"total_publish_requests":0}}]}`,
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
