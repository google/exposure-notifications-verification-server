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
	"fmt"
	"time"
)

// RealmStats represents statistics related to a user in the database.
type RealmStats struct {
	Date         time.Time `gorm:"date; not null"`
	RealmID      uint      `gorm:"realm_id; not null"`
	CodesIssued  uint      `gorm:"codes_issued; default: 0"`
	CodesClaimed uint      `gorm:"codes_claimed; default: 0"`
}

// RealmStatsCSVHeader is a header for CSV files for RealmStats.
var RealmStatsCSVHeader = []string{"Date", "Realm ID", "Codes Issued", "Codes Claimed"}

// CSV returns the CSV encoded values for a RealmStats.
func (r *RealmStats) CSV() []string {
	return []string{
		r.Date.Format("2006-01-02"),
		fmt.Sprintf("%d", r.RealmID),
		fmt.Sprintf("%d", r.CodesIssued),
		fmt.Sprintf("%d", r.CodesClaimed),
	}
}

// TableName sets the RealmStats table name
func (RealmStats) TableName() string {
	return "realm_stats"
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
