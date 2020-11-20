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

// AuditIDStat represents statistics related to a user in the database.
type AuditIDStat struct {
	Date        time.Time `gorm:"date;type:DATE"`
	AuditID     string    `gorm:"audit_id"`
	RealmID     uint      `gorm:"realm_id"`
	CodesIssued uint      `gorm:"codes_issued"`
}

// AuditIDStatsCSVHeader is the header for audit csv requests.
var AuditIDStatsCSVHeader = []string{"Audit ID", "Codes Issued", "Date"}

// TableName sets the UserStats table name
func (AuditIDStat) TableName() string {
	return "audit_id_stats"
}

// CSV returns a slice of the data from a AuditIDStat for CSV writing.
func (s *AuditIDStat) CSV() []string {
	return []string{
		s.AuditID,
		fmt.Sprintf("%d", s.CodesIssued),
		s.Date.Format("2006-01-02"),
	}
}
