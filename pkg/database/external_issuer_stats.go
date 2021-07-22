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
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
)

var _ icsv.Marshaler = (ExternalIssuerStats)(nil)

// ExternalIssuerStats is a collection of external issuer stats.
type ExternalIssuerStats []*ExternalIssuerStat

// ExternalIssuerStat represents statistics related to a user in the database.
type ExternalIssuerStat struct {
	Date        time.Time `gorm:"column:date; type:date;"`
	RealmID     uint      `gorm:"column:realm_id; type:int"`
	IssuerID    string    `gorm:"column:issuer_id; type:varchar(255)"`
	CodesIssued uint      `gorm:"column:codes_issued; type:int;"`
}

// MarshalCSV returns bytes in CSV format.
func (s ExternalIssuerStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "realm_id", "issuer_id", "codes_issued"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.RealmID), 10),
			stat.IssuerID,
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

type jsonExternalIssuerStat struct {
	RealmID uint                           `json:"realm_id"`
	Stats   []*jsonExternalIssuerStatStats `json:"statistics"`
}

type jsonExternalIssuerStatStats struct {
	Date       time.Time                           `json:"date"`
	IssuerData []*jsonExternalIssuerStatIssuerData `json:"issuer_data"`
}

type jsonExternalIssuerStatIssuerData struct {
	IssuerID    string `json:"issuer_id"`
	CodesIssued uint   `json:"codes_issued"`
}

// MarshalJSON is a custom JSON marshaller.
func (s ExternalIssuerStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	m := make(map[time.Time][]*jsonExternalIssuerStatIssuerData)
	for _, stat := range s {
		if m[stat.Date] == nil {
			m[stat.Date] = make([]*jsonExternalIssuerStatIssuerData, 0, 8)
		}

		m[stat.Date] = append(m[stat.Date], &jsonExternalIssuerStatIssuerData{
			IssuerID:    stat.IssuerID,
			CodesIssued: stat.CodesIssued,
		})
	}

	stats := make([]*jsonExternalIssuerStatStats, 0, len(m))
	for k, v := range m {
		stats = append(stats, &jsonExternalIssuerStatStats{
			Date:       k,
			IssuerData: v,
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonExternalIssuerStat
	result.RealmID = s[0].RealmID
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *ExternalIssuerStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonExternalIssuerStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		for _, r := range stat.IssuerData {
			*s = append(*s, &ExternalIssuerStat{
				Date:        stat.Date,
				RealmID:     result.RealmID,
				IssuerID:    r.IssuerID,
				CodesIssued: r.CodesIssued,
			})
		}
	}

	return nil
}

// PurgeExternalIssuerStats will delete stats that were created longer than
// maxAge ago.
func (db *Database) PurgeExternalIssuerStats(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	createdBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("date < ?", createdBefore).
		Delete(&ExternalIssuerStat{})
	return result.RowsAffected, result.Error
}
