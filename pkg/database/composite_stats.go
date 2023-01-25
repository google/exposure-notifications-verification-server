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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

// Type assertions for CompositeStats
var (
	_ icsv.Marshaler = (CompositeStats)(nil)
	_ json.Marshaler = (CompositeStats)(nil)
)

// CompositeStats is an internal type for collecting unifed realm and key server stats.
type CompositeStats []*CompositeDay

// CompositeDay represents a single day of composite stats.
type CompositeDay struct {
	Day            time.Time
	RealmStats     *RealmStat
	KeyServerStats *keyserver.StatsDay
}

func (c *CompositeDay) IsEmpty() bool {
	return c.RealmStats.IsEmpty() && c.KeyServerStats.IsEmpty()
}

type jsonCompositeStat struct {
	RealmID           uint                      `json:"realm_id"`
	HasKeyServerStats bool                      `json:"has_key_server_stats"`
	Stats             []*jsonCompositeStatStats `json:"statistics"`
}

type jsonCompositeStatStats struct {
	Date time.Time                   `json:"date"`
	Data *jsonCompositeStatStatsData `json:"data"`
}

type jsonCompositeStatStatsData struct {
	JSONRealmStatStatsData
	keyserver.StatsDay

	// the only field that isn't from an embedded struct.
	TotalPublishRequests int64 `json:"total_publish_requests"`
}

// MarshalJSON is a custom JSON marshaller.
func (c CompositeStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(c) == 0 {
		return json.Marshal(struct{}{})
	}
	var realmID uint
	hasKeyServerStats := false

	stats := make([]*jsonCompositeStatStats, 0, len(c))
	for _, stat := range c {
		data := &jsonCompositeStatStatsData{}
		if stat.RealmStats != nil {
			if realmID == 0 {
				realmID = stat.RealmStats.RealmID
			}

			data.CodesIssued = stat.RealmStats.CodesIssued
			data.CodesClaimed = stat.RealmStats.CodesClaimed
			data.CodesInvalid = stat.RealmStats.CodesInvalid
			data.CodesInvalidByOS = CodesInvalidByOSData{
				UnknownOS: stat.RealmStats.CodesInvalidByOS[OSTypeUnknown],
				IOS:       stat.RealmStats.CodesInvalidByOS[OSTypeIOS],
				Android:   stat.RealmStats.CodesInvalidByOS[OSTypeAndroid],
			}
			data.UserReportsIssued = stat.RealmStats.UserReportsIssued
			data.UserReportsClaimed = stat.RealmStats.UserReportsClaimed
			data.UserReportsInvalidNonce = stat.RealmStats.UserReportsInvalidNonce
			data.UserReportsInvalidNonceByOS = CodesInvalidByOSData{
				UnknownOS: stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeUnknown],
				IOS:       stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeIOS],
				Android:   stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeAndroid],
			}
			data.TokensClaimed = stat.RealmStats.TokensClaimed
			data.TokensInvalid = stat.RealmStats.TokensInvalid
			data.UserReportTokensClaimed = stat.RealmStats.UserReportTokensClaimed
			data.CodeClaimMeanAge = uint(stat.RealmStats.CodeClaimMeanAge.Duration.Seconds())
			data.CodeClaimDistribution = stat.RealmStats.CodeClaimAgeDistribution
		}
		if stat.KeyServerStats != nil {
			hasKeyServerStats = true
			data.TotalPublishRequests = stat.KeyServerStats.PublishRequests.Total()
			data.PublishRequests = stat.KeyServerStats.PublishRequests
			data.TotalTEKsPublished = stat.KeyServerStats.TotalTEKsPublished
			data.RevisionRequests = stat.KeyServerStats.RevisionRequests
			data.TEKAgeDistribution = stat.KeyServerStats.TEKAgeDistribution
			data.OnsetToUploadDistribution = stat.KeyServerStats.OnsetToUploadDistribution
			data.RequestsMissingOnsetDate = stat.KeyServerStats.RequestsMissingOnsetDate
		}

		stats = append(stats, &jsonCompositeStatStats{
			Date: stat.Day,
			Data: data,
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonCompositeStat
	result.RealmID = realmID
	result.HasKeyServerStats = hasKeyServerStats
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (c *CompositeStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	// This is unused, but required for the interface.
	return nil
}

// MarshalCSV returns bytes in CSV format.
func (c CompositeStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(c) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{
		"date",
		"codes_issued", "codes_claimed", "codes_invalid",
		"tokens_claimed", "tokens_invalid", "code_claim_mean_age_seconds", "code_claim_age_distribution",
		"publish_requests_unknown", "publish_requests_android", "publish_requests_ios",
		"total_teks_published", "requests_with_revisions", "requests_missing_onset_date", "tek_age_distribution", "onset_to_upload_distribution",
		"user_reports_issued", "user_reports_claimed", "user_report_tokens_claimed",
		"codes_invalid_unknown_os", "codes_invalid_ios", "codes_invalid_android",
		"user_reports_invalid_nonce", "user_reports_invalid_nonce_unknown_os", "user_reports_invalid_nonce_ios", "user_reports_invalid_nonce_android",
	}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Due to the nature of the composite stats, one or the other of the branches could be nil
	for i, stat := range c {
		row := make([]string, 0, 16)
		row = append(row, stat.Day.Format(project.RFC3339Date))
		if stat.RealmStats == nil {
			// no realm stats, 7 empty columns
			row = append(row, "", "", "", "", "", "", "")
		} else {
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesIssued), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesClaimed), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesInvalid), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.TokensClaimed), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.TokensInvalid), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodeClaimMeanAge.Duration.Seconds()), 10))
			row = append(row, strings.Join(stat.RealmStats.CodeClaimAgeDistributionAsStrings(), "|"))
		}

		if stat.KeyServerStats == nil {
			// no key sever stats, 8 empty columns
			row = append(row, "", "", "", "", "", "", "", "")
		} else {
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.PublishRequests.UnknownPlatform), 10))
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.PublishRequests.Android), 10))
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.PublishRequests.IOS), 10))
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.TotalTEKsPublished), 10))
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.RevisionRequests), 10))
			row = append(row, strconv.FormatUint(uint64(stat.KeyServerStats.RequestsMissingOnsetDate), 10))
			row = append(row, joinInt64s(stat.KeyServerStats.TEKAgeDistribution, "|"))
			row = append(row, joinInt64s(stat.KeyServerStats.OnsetToUploadDistribution, "|"))
		}

		// User-report stats. Yes, this is the same nil check as above near L168,
		// but we need to preserve the ordering in the CSV for backwards-compat.
		if stat.RealmStats == nil {
			// No user-report stats
			row = append(row, "", "", "")
		} else {
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsIssued), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsClaimed), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportTokensClaimed), 10))
		}

		// Invalid codes by OS
		if stat.RealmStats == nil {
			row = append(row, "", "", "")
		} else {
			// stats day exists, but the array is nil.
			if len(stat.RealmStats.CodesInvalidByOS) == 0 {
				row = append(row, "", "", "")
			} else {
				row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesInvalidByOS[OSTypeUnknown]), 10))
				row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesInvalidByOS[OSTypeIOS]), 10))
				row = append(row, strconv.FormatUint(uint64(stat.RealmStats.CodesInvalidByOS[OSTypeAndroid]), 10))
			}
		}

		if stat.RealmStats == nil {
			row = append(row, "", "", "", "")
		} else {
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsInvalidNonce), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeUnknown]), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeIOS]), 10))
			row = append(row, strconv.FormatUint(uint64(stat.RealmStats.UserReportsInvalidNonceByOS[OSTypeAndroid]), 10))
		}

		// New stats should always be added to the end to preserve existing external user applications.

		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write CSV entry %d: %w", i, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to create CSV: %w", err)
	}

	return b.Bytes(), nil
}

func joinInt64s(arr []int64, sep string) string {
	var sb strings.Builder
	for i, d := range arr {
		sb.WriteString(strconv.FormatInt(d, 10))
		if i != len(arr)-1 {
			sb.WriteString(sep)
		}
	}
	return sb.String()
}
