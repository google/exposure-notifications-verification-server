// Copyright 2021 Google LLC
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
	"time"

	"github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
)

// KeyServerStats represents statistics for a key-server for this realm
type KeyServerStats struct {
	Errorable

	// RealmId that these stats belong to.
	RealmID uint `gorm:"column:realm_id; primary_key; type:integer; not null;"`

	// IsSystem determines if this is a system-level stats configuration. There can
	// only be one system-level stats configuration.
	IsSystem bool `gorm:"column:is_system; type:bool; not null; default:false;"`

	// KeyServerURL allows a realm to override the system's URL with its own
	KeyServerURL string `gorm:"column:key_server_url; type:varchar(150); default: '';"`
	// KeyServerAudience allows a realm to override the system's audience
	KeyServerAudience string `gorm:"column:key_server_audience; type:varchar(150); default: '';"`

	Stats []*KeyServerStatsDay `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false;"`
}

// KeyServerStatsDay represents statistics for each day
type KeyServerStatsDay struct {
	Errorable

	// RealmId that these stats belong to.
	RealmID uint `gorm:"column:realm_id; primary_key; type:integer; not null;"`

	// Day will be set to midnight UTC of the day represented. An individual day
	// isn't released until there is a minimum threshold for updates has been met.
	Day time.Time `gorm:"column:day; primary_key;"`

	// PublishRequests is a count of requests per OS
	PublishRequests postgres.Hstore `gorm:"column:publish_requests; type:hstore;"`

	TotalTEKsPublished int64 `gorm:"column:total_teks_published; type:bigint; not null; default: 0;"`

	// RevisionRequests is the number of publish requests that contained at least one TEK revision.
	RevisionRequests int64 `gorm:"column:revision_requests; type:bigint; not null; default: 0;"`

	// TEKAgeDistribution shows a distribution of the oldest tek in an upload.
	// The count at index 0-15 represent the number of uploads there the oldest TEK is that value.
	// Index 16 represents > 15 days.
	TEKAgeDistribution pq.Int64Array `gorm:"column:tek_age_distribution; type:bigint[];"`

	// OnsetToUploadDistribution shows a distribution of onset to upload, the index is in days.
	// The count at index 0-29 represents the number of uploads with that symptom onset age.
	// Index 30 represents > 29 days.
	OnsetToUploadDistribution pq.Int64Array `gorm:"column:onset_to_upload_distribution; type:bigint[];"`

	// RequestsMissingOnsetDate is the number of publish requests where no onset date
	// was provided. These request are not included in the onset to upload distribution.
	RequestsMissingOnsetDate int64 `gorm:"column:request_missing_onset_date; type:bigint; not null; default: 0;"`
}

func (db *Database) GetKeyServerStats(realmID uint) (*KeyServerStats, error) {
	var stats KeyServerStats
	if err := db.db.
		Where("realm_id = ?", realmID).
		First(&stats).
		Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

func (db *Database) SaveKeyServerStats(stats *KeyServerStats) error {
	return db.db.Save(stats).Error
}

func (db *Database) GetKeyServerStatsDay(realmID uint, day time.Time) (*KeyServerStatsDay, error) {
	var stats KeyServerStatsDay
	if err := db.db.
		Where("realm_id = ?", realmID).
		Where("day = ?", day).
		First(&stats).
		Error; err != nil {
		return nil, err
	}
	return &stats, nil
}

func (db *Database) SaveKeyServerStatsDay(day *KeyServerStatsDay) error {
	return db.db.Save(day).Error
}
