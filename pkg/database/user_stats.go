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
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/jinzhu/gorm"
)

var _ icsv.Marshaler = (UserStats)(nil)

// UserStats represents a logical collection of stats for a user.
type UserStats []*UserStat

// UserStat represents a single-date statistic for a user.
type UserStat struct {
	Date        time.Time `gorm:"date; not null;"`
	UserID      uint      `gorm:"user_id; not null;"`
	RealmID     uint      `gorm:"realm_id; default:0;"`
	CodesIssued uint      `gorm:"codes_issued; default:0;"`

	// Non-database fields, these are added via the stats lookup using the join
	// table.
	UserName  string `gorm:"-"`
	UserEmail string `gorm:"-"`
}

// MarshalCSV returns bytes in CSV format.
func (s UserStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "realm_id", "user_id", "user_name", "user_email", "codes_issued"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.RealmID), 10),
			strconv.FormatUint(uint64(stat.UserID), 10),
			stat.UserName,
			stat.UserEmail,
			strconv.FormatUint(uint64(stat.CodesIssued), 10),
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

type jsonUserStat struct {
	RealmID   uint                 `json:"realm_id"`
	UserID    uint                 `json:"user_id"`
	UserName  string               `json:"user_name"`
	UserEmail string               `json:"user_email"`
	Stats     []*jsonUserStatStats `json:"statistics"`
}

type jsonUserStatStats struct {
	Date time.Time              `json:"date"`
	Data *jsonUserStatStatsData `json:"data"`
}

type jsonUserStatStatsData struct {
	CodesIssued uint `json:"codes_issued"`
}

// MarshalJSON is a custom JSON marshaller.
func (s UserStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	stats := make([]*jsonUserStatStats, 0, len(s))
	for _, stat := range s {
		stats = append(stats, &jsonUserStatStats{
			Date: stat.Date,
			Data: &jsonUserStatStatsData{
				CodesIssued: stat.CodesIssued,
			},
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonUserStat
	result.RealmID = s[0].RealmID
	result.UserID = s[0].UserID
	result.UserName = s[0].UserName
	result.UserEmail = s[0].UserEmail
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *UserStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonUserStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		*s = append(*s, &UserStat{
			Date:        stat.Date,
			RealmID:     result.RealmID,
			UserID:      result.UserID,
			UserName:    result.UserName,
			UserEmail:   result.UserEmail,
			CodesIssued: stat.Data.CodesIssued,
		})
	}

	return nil
}

// SaveUserStat saves some UserStats to the database. This function is provided
// for testing only.
func (db *Database) SaveUserStat(u *UserStat) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(u).Error; err != nil {
			return err
		}
		return nil
	})
}
