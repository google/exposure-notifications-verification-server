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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/lib/pq"
)

var _ icsv.Marshaler = (RealmStats)(nil)

// RealmStats represents a logical collection of stats of a realm.
type RealmStats []*RealmStat

var claimDistributionBuckets = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour,
	2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 336 * time.Hour,
}

// RealmStat represents statistics related to a user in the database.
type RealmStat struct {
	Date    time.Time `gorm:"column:date; type:date; not null;"`
	RealmID uint      `gorm:"column:realm_id; type:integer; not null;"`

	// CodesIssued is the total number of codes issued. CodesClaimed are
	// successful claims. CodesInvalid are codes that have failed to claim
	// (expired or not found).
	CodesIssued  uint `gorm:"column:codes_issued; type:integer; not null; default:0;"`
	CodesClaimed uint `gorm:"column:codes_claimed; type:integer; not null; default:0;"`
	CodesInvalid uint `gorm:"column:codes_invalid; type:integer; not null; default:0;"`

	// TokensClaimed is the number of tokens exchanged for a certificate.
	// TokensInvalid is the number of tokens which failed to exchange due to
	// a user error.
	TokensClaimed uint `gorm:"column:tokens_claimed; type:integer; not null; default:0;"`
	TokensInvalid uint `gorm:"column:tokens_invalid; type:integer; not null; default:0;"`

	// CodeClaimAgeDistribution shows a distribution of time from code issue to claim.
	// Buckets are: 1m, 5m, 15m, 30m, 1h, 2h, 3h, 6h, 12h, 24h, >24h
	CodeClaimAgeDistribution pq.Int32Array `gorm:"column:code_claim_age_distribution; type:int[];"`

	// CodeClaimMeanAge tracks the average age to claim a code.
	CodeClaimMeanAge DurationSeconds `gorm:"column:code_claim_mean_age; type:bigint; not null; default: 0;"`
}

func (s *RealmStat) CodeClaimAgeDistributionAsStrings() []string {
	str := make([]string, len(s.CodeClaimAgeDistribution))
	for i, v := range s.CodeClaimAgeDistribution {
		str[i] = strconv.Itoa(int(v))
	}
	return str
}

// MarshalCSV returns bytes in CSV format.
func (s RealmStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{
		"date",
		"codes_issued", "codes_claimed", "codes_invalid",
		"tokens_claimed", "tokens_invalid", "code_claim_mean_age_seconds", "code_claim_age_distribution",
	}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.CodesIssued), 10),
			strconv.FormatUint(uint64(stat.CodesClaimed), 10),
			strconv.FormatUint(uint64(stat.CodesInvalid), 10),
			strconv.FormatUint(uint64(stat.TokensClaimed), 10),
			strconv.FormatUint(uint64(stat.TokensInvalid), 10),
			strconv.FormatUint(uint64(stat.CodeClaimMeanAge.Duration.Seconds()), 10),
			join(stat.CodeClaimAgeDistribution, "|"),
		}); err != nil {
			return nil, fmt.Errorf("failed to write CSV entry %d: %w", i, err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to create CSV: %w", err)
	}

	return b.Bytes(), nil
}

func join(arr []int32, sep string) string {
	var sb strings.Builder
	for i, d := range arr {
		sb.WriteString(strconv.Itoa(int(d)))
		if i != len(arr)-1 {
			sb.WriteString(sep)
		}
	}
	return sb.String()
}

type jsonRealmStat struct {
	RealmID uint                  `json:"realm_id"`
	Stats   []*jsonRealmStatStats `json:"statistics"`
}

type jsonRealmStatStats struct {
	Date time.Time               `json:"date"`
	Data *jsonRealmStatStatsData `json:"data"`
}

type jsonRealmStatStatsData struct {
	CodesIssued           uint    `json:"codes_issued"`
	CodesClaimed          uint    `json:"codes_claimed"`
	CodesInvalid          uint    `json:"codes_invalid"`
	TokensClaimed         uint    `json:"tokens_claimed"`
	TokensInvalid         uint    `json:"tokens_invalid"`
	CodeClaimMeanAge      uint    `json:"code_claim_mean_age_seconds"`
	CodeClaimDistribution []int32 `json:"code_claim_age_distribution"`
}

// MarshalJSON is a custom JSON marshaller.
func (s RealmStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	stats := make([]*jsonRealmStatStats, 0, len(s))
	for _, stat := range s {
		stats = append(stats, &jsonRealmStatStats{
			Date: stat.Date,
			Data: &jsonRealmStatStatsData{
				CodesIssued:           stat.CodesIssued,
				CodesClaimed:          stat.CodesClaimed,
				CodesInvalid:          stat.CodesInvalid,
				TokensClaimed:         stat.TokensClaimed,
				TokensInvalid:         stat.TokensInvalid,
				CodeClaimMeanAge:      uint(stat.CodeClaimMeanAge.Duration.Seconds()),
				CodeClaimDistribution: stat.CodeClaimAgeDistribution,
			},
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonRealmStat
	result.RealmID = s[0].RealmID
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *RealmStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonRealmStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		*s = append(*s, &RealmStat{
			Date:                     stat.Date,
			RealmID:                  result.RealmID,
			CodesIssued:              stat.Data.CodesIssued,
			CodesClaimed:             stat.Data.CodesClaimed,
			CodesInvalid:             stat.Data.CodesInvalid,
			TokensClaimed:            stat.Data.TokensClaimed,
			TokensInvalid:            stat.Data.TokensInvalid,
			CodeClaimMeanAge:         FromDuration(time.Duration(stat.Data.CodeClaimMeanAge) * time.Second),
			CodeClaimAgeDistribution: stat.Data.CodeClaimDistribution,
		})
	}

	return nil
}

// HistoricalCodesIssued returns a slice of the historical codes issued for
// this realm by date descending.
func (r *Realm) HistoricalCodesIssued(db *Database, limit uint64) ([]uint64, error) {
	var stats []uint64
	if err := db.db.
		Model(&RealmStats{}).
		Where("realm_id = ?", r.ID).
		Order("date DESC").
		Limit(limit).
		Pluck("codes_issued", &stats).
		Error; err != nil {
		return nil, err
	}
	return stats, nil
}
