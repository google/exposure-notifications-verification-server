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
	"github.com/jinzhu/gorm"
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
	// (expired or not found). This includes UserReportsIssued and
	// UserReportsClaimed.
	CodesIssued  uint `gorm:"column:codes_issued; type:integer; not null; default:0;"`
	CodesClaimed uint `gorm:"column:codes_claimed; type:integer; not null; default:0;"`
	CodesInvalid uint `gorm:"column:codes_invalid; type:integer; not null; default:0;"`

	// CodesInvalidByOS is an array where the index is the controller.OperatingSystem enums.
	CodesInvalidByOS pq.Int64Array `gorm:"column:codes_invalid_by_os; type:bigint[];"`

	// UserReportsIssued is the specific number of codes that were issued because
	// the user initiated a self-report request. These numbers are also included
	// in the sum of codes issued and codes claimed.
	UserReportsIssued  uint `gorm:"column:user_reports_issued; type:integer; not null; default:0;"`
	UserReportsClaimed uint `gorm:"column:user_reports_claimed; type:integer; not null; default:0;"`
	UserReportsInvalid uint `gorm:"column:user_reports_invalid; type:integer; not null; default:0;"`

	// TokensClaimed is the number of tokens exchanged for a certificate.
	// TokensInvalid is the number of tokens which failed to exchange due to
	// a user error. This includes UserReportTokensClaimed.
	TokensClaimed uint `gorm:"column:tokens_claimed; type:integer; not null; default:0;"`
	TokensInvalid uint `gorm:"column:tokens_invalid; type:integer; not null; default:0;"`

	// UserReportTokensClaimed is the number of tokens claimed that represent a user
	// initiated report. This sum is also included in tokens claimed.
	UserReportTokensClaimed uint `gorm:"column:user_report_tokens_claimed; type:integer; not null; default:0;"`

	// CodeClaimAgeDistribution shows a distribution of time from code issue to claim.
	// Buckets are: 1m, 5m, 15m, 30m, 1h, 2h, 3h, 6h, 12h, 24h, >24h
	CodeClaimAgeDistribution pq.Int32Array `gorm:"column:code_claim_age_distribution; type:int[];"`

	// CodeClaimMeanAge tracks the average age to claim a code.
	CodeClaimMeanAge DurationSeconds `gorm:"column:code_claim_mean_age; type:bigint; not null; default: 0;"`
}

func (s *RealmStat) IsEmpty() bool {
	if s == nil {
		return true
	}

	if s.CodesIssued > 0 {
		return false
	}
	if s.CodesClaimed > 0 {
		return false
	}
	if s.CodesInvalid > 0 {
		return false
	}
	if s.UserReportsIssued > 0 {
		return false
	}
	if s.UserReportsClaimed > 0 {
		return false
	}
	if s.UserReportsInvalid > 0 {
		return false
	}
	if s.TokensClaimed > 0 {
		return false
	}
	if s.TokensInvalid > 0 {
		return false
	}
	if s.UserReportTokensClaimed > 0 {
		return false
	}

	for _, v := range s.CodeClaimAgeDistribution {
		if v > 0 {
			return false
		}
	}

	return true
}

func (s *RealmStat) AfterFind(tx *gorm.DB) (err error) {
	if len(s.CodesInvalidByOS) == 0 {
		s.CodesInvalidByOS = make([]int64, OSTypeUnknown.Len())
	}
	return nil
}

// CodeClaimAgeDistributionAsStrings returns CodeClaimAgeDistribution as
// []string instead of []int32. Useful for serialization.
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
		"user_reports_issued", "user_reports_claimed", "user_report_tokens_claimed",
		"codes_invalid_unknown_os", "codes_invalid_ios", "codes_invalid_android",
		"user_reports_invalid",
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
			strconv.FormatUint(uint64(stat.UserReportsIssued), 10),
			strconv.FormatUint(uint64(stat.UserReportsClaimed), 10),
			strconv.FormatUint(uint64(stat.UserReportTokensClaimed), 10),
			strconv.FormatUint(uint64(stat.CodesInvalidByOS[OSTypeUnknown]), 10),
			strconv.FormatUint(uint64(stat.CodesInvalidByOS[OSTypeIOS]), 10),
			strconv.FormatUint(uint64(stat.CodesInvalidByOS[OSTypeAndroid]), 10),
			strconv.FormatUint(uint64(stat.UserReportsInvalid), 10),
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
	Data *JSONRealmStatStatsData `json:"data"`
}

type CodesInvalidByOSData struct {
	UnknownOS int64 `json:"unknown_os"`
	IOS       int64 `json:"ios"`
	Android   int64 `json:"android"`
}

type JSONRealmStatStatsData struct {
	CodesIssued             uint                 `json:"codes_issued"`
	CodesClaimed            uint                 `json:"codes_claimed"`
	CodesInvalid            uint                 `json:"codes_invalid"`
	CodesInvalidByOS        CodesInvalidByOSData `json:"codes_invalid_by_os"`
	UserReportsIssued       uint                 `json:"user_reports_issued"`
	UserReportsClaimed      uint                 `json:"user_reports_claimed"`
	UserReportsInvalid      uint                 `json:"user_reports_invalid"`
	TokensClaimed           uint                 `json:"tokens_claimed"`
	TokensInvalid           uint                 `json:"tokens_invalid"`
	UserReportTokensClaimed uint                 `json:"user_report_tokens_claimed"`
	CodeClaimMeanAge        uint                 `json:"code_claim_mean_age_seconds"`
	CodeClaimDistribution   []int32              `json:"code_claim_age_distribution"`
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
			Data: &JSONRealmStatStatsData{
				CodesIssued:  stat.CodesIssued,
				CodesClaimed: stat.CodesClaimed,
				CodesInvalid: stat.CodesInvalid,
				CodesInvalidByOS: CodesInvalidByOSData{
					UnknownOS: stat.CodesInvalidByOS[OSTypeUnknown],
					IOS:       stat.CodesInvalidByOS[OSTypeIOS],
					Android:   stat.CodesInvalidByOS[OSTypeAndroid],
				},
				UserReportsIssued:       stat.UserReportsIssued,
				UserReportsClaimed:      stat.UserReportsClaimed,
				UserReportsInvalid:      stat.UserReportsInvalid,
				TokensClaimed:           stat.TokensClaimed,
				TokensInvalid:           stat.TokensInvalid,
				UserReportTokensClaimed: stat.UserReportTokensClaimed,
				CodeClaimMeanAge:        uint(stat.CodeClaimMeanAge.Duration.Seconds()),
				CodeClaimDistribution:   stat.CodeClaimAgeDistribution,
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
			Date:         stat.Date,
			RealmID:      result.RealmID,
			CodesIssued:  stat.Data.CodesIssued,
			CodesClaimed: stat.Data.CodesClaimed,
			CodesInvalid: stat.Data.CodesInvalid,
			CodesInvalidByOS: []int64{
				stat.Data.CodesInvalidByOS.UnknownOS,
				stat.Data.CodesInvalidByOS.IOS,
				stat.Data.CodesInvalidByOS.Android,
			},
			UserReportsIssued:        stat.Data.UserReportsIssued,
			UserReportsClaimed:       stat.Data.UserReportsClaimed,
			UserReportsInvalid:       stat.Data.UserReportsInvalid,
			TokensClaimed:            stat.Data.TokensClaimed,
			TokensInvalid:            stat.Data.TokensInvalid,
			UserReportTokensClaimed:  stat.Data.UserReportTokensClaimed,
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

// PurgeRealmStats will delete stats that were created longer than
// maxAge ago.
func (db *Database) PurgeRealmStats(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	createdBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("date < ?", createdBefore).
		Delete(&RealmStat{})
	return result.RowsAffected, result.Error
}
