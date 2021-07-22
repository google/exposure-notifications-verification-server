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
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
)

// RealmChaffEvent is a record that indicates a realm received a chaff event on
// the given date.
type RealmChaffEvent struct {
	// RealmID is the realm for which the chaff request existed.
	RealmID uint

	// Date is the UTC date (truncated to midnight) for which one or more chaff
	// request existed.
	Date time.Time

	// Present indicates whether the chaff event was present.
	Present bool
}

// HasRealmChaffEventsMap returns a map of realm IDs that have any chaff events.
func (db *Database) HasRealmChaffEventsMap() (map[uint]bool, error) {
	var ids []uint
	if err := db.db.
		Model(&RealmChaffEvent{}).
		Select("DISTINCT(realm_id) AS realm_id").
		Pluck("realm_id", &ids).
		Error; err != nil {
		return nil, fmt.Errorf("failed to pluck ids: %w", err)
	}

	// This has to be a bool because go text/template doesn't work with struct{}.
	m := make(map[uint]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

// PurgeRealmChaffEvents will delete realm chaff events that have exceeded the
// storage lifetime.
func (db *Database) PurgeRealmChaffEvents(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := timeutils.UTCMidnight(time.Now().UTC()).Add(maxAge)

	result := db.db.
		Unscoped().
		Where("date < ?", deleteBefore).
		Delete(&RealmChaffEvent{})
	return result.RowsAffected, result.Error
}
