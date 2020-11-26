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
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
)

var _ icsv.Marshaler = (RealmStats)(nil)

// RealmStats represents a logical collection of stats of a realm.
type RealmStats []*RealmStat

// RealmStat represents statistics related to a user in the database.
type RealmStat struct {
	Date         time.Time `gorm:"date; not null;"`
	RealmID      uint      `gorm:"realm_id; not null;"`
	CodesIssued  uint      `gorm:"codes_issued; default:0;"`
	CodesClaimed uint      `gorm:"codes_claimed; default:0;"`
}

// MarshalCSV returns bytes in CSV format.
func (s RealmStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "codes_issued", "codes_claimed"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format("2006-01-02"),
			strconv.FormatUint(uint64(stat.CodesIssued), 10),
			strconv.FormatUint(uint64(stat.CodesClaimed), 10),
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

type jsonRealmStat struct {
	RealmID uint                  `json:"realm_id"`
	Stats   []*jsonRealmStatStats `json:"statistics"`
}

type jsonRealmStatStats struct {
	Date time.Time               `json:"date"`
	Data *jsonRealmStatStatsData `json:"data"`
}

type jsonRealmStatStatsData struct {
	CodesIssued  uint `json:"codes_issued"`
	CodesClaimed uint `json:"codes_claimed"`
}

// MarshalJSON is a custom JSON marshaller.
func (s RealmStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	var stats []*jsonRealmStatStats
	for _, stat := range s {
		stats = append(stats, &jsonRealmStatStats{
			Date: stat.Date,
			Data: &jsonRealmStatStatsData{
				CodesIssued:  stat.CodesIssued,
				CodesClaimed: stat.CodesClaimed,
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
