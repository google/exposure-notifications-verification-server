// Copyright 2021 Google LLC
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

package stats

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
)

// Type assertions for CompositeStats
var _ icsv.Marshaler = (CompositeStats)(nil)
var _ json.Marshaler = (CompositeStats)(nil)

// CompositeStats is an internal type for collecting unifed realm and key server stats.
type CompositeStats []*CompositeDay

// CompositeData represents a single day of composite stats.
type CompositeDay struct {
	Day            time.Time
	RealmStats     *database.RealmStat
	KeyServerStats *keyserver.StatsDay
}

type jsonCompositeStat struct {
	RealmID uint                      `json:"realm_id"`
	Stats   []*jsonCompositeStatStats `json:"statistics"`
}

type jsonCompositeStatStats struct {
	Date time.Time                   `json:"date"`
	Data *jsonCompositeStatStatsData `json:"data"`
}

type jsonCompositeStatStatsData struct {
	// Fields that come from the realm stats
	CodesIssued           uint    `json:"codes_issued"`
	CodesClaimed          uint    `json:"codes_claimed"`
	CodesInvalid          uint    `json:"codes_invalid"`
	TokensClaimed         uint    `json:"tokens_claimed"`
	TokensInvalid         uint    `json:"tokens_invalid"`
	CodeClaimMeanAge      uint    `json:"code_claim_mean_age_seconds"`
	CodeClaimDistribution []int32 `json:"code_claim_age_distribution"`

	// Fields that come from the key server stats
	TotalPublishRequests int64                     `json:"total_publish_requests"`
	PublishRequests      keyserver.PublishRequests `json:"publish_requests"`
	TotalTEKsPublished   int64                     `json:"total_teks_published"`
	// RevisionRequests is the number of publish requests that contained at least one TEK revision.
	RevisionRequests int64 `json:"requests_with_revisions"`
	// TEKAgeDistribution shows a distribution of the oldest tek in an upload.
	// The count at index 0-15 represent the number of uploads there the oldest TEK is that value.
	// Index 16 represents > 15 days.
	TEKAgeDistribution []int64 `json:"tek_age_distribution"`
	// OnsetToUploadDistribution shows a distribution of onset to upload, the index is in days.
	// The count at index 0-29 represents the number of uploads with that symptom onset age.
	// Index 30 represents > 29 days.
	OnsetToUploadDistribution []int64 `json:"onset_to_upload_distribution"`

	// RequestsMissingOnsetDate is the number of publish requests where no onset date
	// was provided. These request are not included in the onset to upload distribution.
	RequestsMissingOnsetDate int64 `json:"requests_missing_onset_date"`
}

// MarshalJSON is a custom JSON marshaller.
func (c CompositeStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(c) == 0 {
		return json.Marshal(struct{}{})
	}
	var realmID uint

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
			data.TokensClaimed = stat.RealmStats.TokensClaimed
			data.TokensInvalid = stat.RealmStats.TokensInvalid
			data.CodeClaimMeanAge = uint(stat.RealmStats.CodeClaimMeanAge.Duration.Seconds())
			data.CodeClaimDistribution = stat.RealmStats.CodeClaimAgeDistribution
		}
		if stat.KeyServerStats != nil {
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
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *CompositeStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	// This is unused, but required for the interface.
	return nil
}

// TODO(mikehelmick) - Make this join part of the public API in key sever, then remove this
func join(arr []int64, sep string) string {
	var sb strings.Builder
	for i, d := range arr {
		sb.WriteString(strconv.FormatInt(d, 10))
		if i != len(arr)-1 {
			sb.WriteString(sep)
		}
	}
	return sb.String()
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
			row = append(row, join(stat.KeyServerStats.TEKAgeDistribution, "|"))
			row = append(row, join(stat.KeyServerStats.OnsetToUploadDistribution, "|"))
		}

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

// HandleComposite returns composite states for realm + key server
// The key server stats may be omitted if that is not enabled
// on the realm.
func (c *Controller) HandleComposite(typ Type) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		currentRealm, ok := authorizeFromContext(ctx, rbac.StatsRead)
		if !ok {
			controller.Unauthorized(w, r, c.h)
			return
		}

		realmStats, err := currentRealm.StatsCached(ctx, c.db, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var stats CompositeStats = make([]*CompositeDay, 0, len(realmStats))
		statsMap := make(map[time.Time]*CompositeDay, len(realmStats))
		for _, rs := range realmStats {
			day := &CompositeDay{
				Day:        rs.Date,
				RealmStats: rs,
			}
			stats = append(stats, day)
			statsMap[rs.Date] = day
		}

		if c.db.HasKeyServerStats(currentRealm.ID) {
			days, err := c.db.ListKeyServerStatsDaysCached(ctx, currentRealm.ID, c.cacher)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			needsSort := false
			for _, ksDay := range days {
				compDay, ok := statsMap[ksDay.Day]
				if !ok {
					// if key server has stats from a day the realm doesn't, add it in.
					compDay := &CompositeDay{
						Day: ksDay.Day,
					}
					stats = append(stats, compDay)
					needsSort = true
				}
				compDay.KeyServerStats = ksDay.ToResponse()
			}

			if needsSort {
				sort.Slice(stats, func(i, j int) bool {
					return stats[i].Day.Before(stats[j].Day)
				})
			}
		}

		switch typ {
		case TypeCSV:
			c.h.RenderCSV(w, http.StatusOK, csvFilename("composite-stats"), stats)
			return
		case TypeJSON:
			c.h.RenderJSON(w, http.StatusOK, stats)
			return
		default:
			controller.NotFound(w, r, c.h)
			return
		}
	})
}
