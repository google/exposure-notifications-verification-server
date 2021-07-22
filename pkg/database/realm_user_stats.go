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

var _ icsv.Marshaler = (RealmUserStats)(nil)

// RealmUserStats is a grouping collection of RealmUserStat.
type RealmUserStats []*RealmUserStat

// RealmUserStat is an interim data structure representing a single date/user
// statistic. It does not correspond to a single database table, but is rather a
// join across multiple tables.
type RealmUserStat struct {
	Date        time.Time
	RealmID     uint
	UserID      uint
	Name        string
	Email       string
	CodesIssued uint
}

// MarshalCSV returns bytes in CSV format.
func (s RealmUserStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "realm_id", "user_id", "name", "email", "codes_issued"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.RealmID), 10),
			strconv.FormatUint(uint64(stat.UserID), 10),
			stat.Name,
			stat.Email,
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

type jsonRealmUserStat struct {
	RealmID uint                      `json:"realm_id"`
	Stats   []*jsonRealmUserStatStats `json:"statistics"`
}

type jsonRealmUserStatStats struct {
	Date       time.Time                      `json:"date"`
	IssuerData []*jsonRealmUserStatIssuerData `json:"issuer_data"`
}

type jsonRealmUserStatIssuerData struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	CodesIssued uint   `json:"codes_issued"`
}

// MarshalJSON is a custom JSON marshaller.
func (s RealmUserStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	m := make(map[time.Time][]*jsonRealmUserStatIssuerData)
	for _, stat := range s {
		if m[stat.Date] == nil {
			m[stat.Date] = make([]*jsonRealmUserStatIssuerData, 0, 8)
		}

		m[stat.Date] = append(m[stat.Date], &jsonRealmUserStatIssuerData{
			UserID:      stat.UserID,
			Name:        stat.Name,
			Email:       stat.Email,
			CodesIssued: stat.CodesIssued,
		})
	}

	stats := make([]*jsonRealmUserStatStats, 0, len(m))
	for k, v := range m {
		stats = append(stats, &jsonRealmUserStatStats{
			Date:       k,
			IssuerData: v,
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonRealmUserStat
	result.RealmID = s[0].RealmID
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *RealmUserStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonRealmUserStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		for _, r := range stat.IssuerData {
			*s = append(*s, &RealmUserStat{
				Date:        stat.Date,
				RealmID:     result.RealmID,
				UserID:      r.UserID,
				Name:        r.Name,
				Email:       r.Email,
				CodesIssued: r.CodesIssued,
			})
		}
	}

	return nil
}
