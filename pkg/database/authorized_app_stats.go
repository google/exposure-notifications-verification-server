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
)

var _ icsv.Marshaler = (AuthorizedAppStats)(nil)

// AuthorizedAppStats represents a logical collection of stats for an authorized
// app.
type AuthorizedAppStats []*AuthorizedAppStat

// AuthorizedAppStat represents statistics related to an API key in the
// database.
type AuthorizedAppStat struct {
	Date            time.Time `gorm:"date; not null;"`
	AuthorizedAppID uint      `gorm:"authorized_app_id; not null;"`
	CodesIssued     uint      `gorm:"codes_issued; default: 0;"`

	// Non-database fields, these are added via the stats lookup using the join
	// table.
	AuthorizedAppName string `gorm:"-"`
}

// MarshalCSV returns bytes in CSV format.
func (s AuthorizedAppStats) MarshalCSV() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	w := csv.NewWriter(&b)

	if err := w.Write([]string{"date", "authorized_app_id", "authorized_app_name", "codes_issued"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for i, stat := range s {
		if err := w.Write([]string{
			stat.Date.Format(project.RFC3339Date),
			strconv.FormatUint(uint64(stat.AuthorizedAppID), 10),
			stat.AuthorizedAppName,
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

type jsonAuthorizedAppStat struct {
	AuthorizedAppID   uint                          `json:"authorized_app_id"`
	AuthorizedAppName string                        `json:"authorized_app_name"`
	Stats             []*jsonAuthorizedAppStatstats `json:"statistics"`
}

type jsonAuthorizedAppStatstats struct {
	Date time.Time                       `json:"date"`
	Data *jsonAuthorizedAppStatstatsData `json:"data"`
}

type jsonAuthorizedAppStatstatsData struct {
	CodesIssued uint `json:"codes_issued"`
}

// MarshalJSON is a custom JSON marshaller.
func (s AuthorizedAppStats) MarshalJSON() ([]byte, error) {
	// Do nothing if there's no records
	if len(s) == 0 {
		return json.Marshal(struct{}{})
	}

	var stats []*jsonAuthorizedAppStatstats
	for _, stat := range s {
		stats = append(stats, &jsonAuthorizedAppStatstats{
			Date: stat.Date,
			Data: &jsonAuthorizedAppStatstatsData{
				CodesIssued: stat.CodesIssued,
			},
		})
	}

	// Sort in descending order.
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.After(stats[j].Date)
	})

	var result jsonAuthorizedAppStat
	result.AuthorizedAppID = s[0].AuthorizedAppID
	result.AuthorizedAppName = s[0].AuthorizedAppName
	result.Stats = stats

	b, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json: %w", err)
	}
	return b, nil
}

func (s *AuthorizedAppStats) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}

	var result jsonAuthorizedAppStat
	if err := json.Unmarshal(b, &result); err != nil {
		return err
	}

	for _, stat := range result.Stats {
		*s = append(*s, &AuthorizedAppStat{
			Date:              stat.Date,
			AuthorizedAppID:   result.AuthorizedAppID,
			AuthorizedAppName: result.AuthorizedAppName,
			CodesIssued:       stat.Data.CodesIssued,
		})
	}

	return nil
}
