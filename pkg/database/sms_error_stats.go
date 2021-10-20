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
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

var _ icsv.Marshaler = (SMSErrorStats)(nil)

// SMSErrorStats is a collection of external issuer stats.
type SMSErrorStats []*SMSErrorStat

// SMSErrorStat represents statistics related to a user in the database.
type SMSErrorStat struct {
	Date      time.Time `gorm:"column:date; type:date;"`
	RealmID   uint      `gorm:"column:realm_id; type:int;"`
	ErrorCode string    `gorm:"column:error_code; type:text;"`
	Quantity  uint      `gorm:"column:quantity; type:int;"`
}

// InsertSMSErrorStat inserts a new SMS error stat for the given realm and error
// code.
func (db *Database) InsertSMSErrorStat(realmID uint, errorCode string) error {
	date := timeutils.UTCMidnight(time.Now())

	sql := `
		INSERT INTO sms_error_stats (date, realm_id, error_code, quantity)
			VALUES ($1, $2, $3, 1)
		ON CONFLICT (date, realm_id, error_code) DO UPDATE
			SET quantity = sms_error_stats.quantity + 1
	`

	if err := db.db.Exec(sql, date, realmID, errorCode).Error; err != nil {
		return fmt.Errorf("failed to insert sms error stats: %w", err)
	}
	return nil
}

// MarshalCSV returns bytes in CSV format.
func (s SMSErrorStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "realm_id", "error_code", "quantity"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.RealmID), 10),
			stat.ErrorCode,
			strconv.FormatUint(uint64(stat.Quantity), 10),
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

type jsonSMSErrorStat struct {
	RealmID uint                     `json:"realm_id"`
	Stats   []*jsonSMSErrorStatstats `json:"statistics"`
}

type jsonSMSErrorStatstats struct {
	Date      time.Time                    `json:"date"`
	ErrorData []*jsonSMSErrorStatErrorData `json:"error_data"`
}

type jsonSMSErrorStatErrorData struct {
	ErrorCode string `json:"error_code"`
	Quantity  uint   `json:"quantity"`
}

// MarshalJSON is a custom JSON marshaller.
func (s SMSErrorStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	m := make(map[time.Time][]*jsonSMSErrorStatErrorData)
	for _, stat := range s {
		if m[stat.Date] == nil {
			m[stat.Date] = make([]*jsonSMSErrorStatErrorData, 0, 8)
		}

		m[stat.Date] = append(m[stat.Date], &jsonSMSErrorStatErrorData{
			ErrorCode: stat.ErrorCode,
			Quantity:  stat.Quantity,
		})
	}

	stats := make([]*jsonSMSErrorStatstats, 0, len(m))
	for k, v := range m {
		stats = append(stats, &jsonSMSErrorStatstats{
			Date:      k,
			ErrorData: v,
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonSMSErrorStat
	result.RealmID = s[0].RealmID
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *SMSErrorStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonSMSErrorStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		for _, r := range stat.ErrorData {
			*s = append(*s, &SMSErrorStat{
				Date:      stat.Date,
				RealmID:   result.RealmID,
				ErrorCode: r.ErrorCode,
				Quantity:  r.Quantity,
			})
		}
	}

	return nil
}

// PurgeSMSErrorStats will delete stats that were created longer than
// maxAge ago.
func (db *Database) PurgeSMSErrorStats(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	createdBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("date < ?", createdBefore).
		Delete(&SMSErrorStat{})
	return result.RowsAffected, result.Error
}
