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

	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
)

// KeyServerStats represents statistics for a key-server for this realm
type KeyServerStats struct {
	Errorable

	// RealmId that these stats belong to.
	RealmID uint `gorm:"column:realm_id; primary_key; type:integer; not null;"`

	// KeyServerURL allows a realm to override the system's URL with its own
	KeyServerURL string `gorm:"column:key_server_url; type:text;"`
	// KeyServerAudience allows a realm to override the system's audience
	KeyServerAudience string `gorm:"column:key_server_audience; type:text;"`
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
	// where the index corresponds to the value of OSType
	PublishRequests pq.Int64Array `gorm:"column:publish_requests; type:bigint[];"`

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

// BeforeSave runs validations. If there are errors, the save fails.
func (kss *KeyServerStats) BeforeSave(tx *gorm.DB) error {
	if kss.RealmID == 0 && (kss.KeyServerURL == "" || kss.KeyServerAudience == "") {
		kss.AddError("realm_id", "the system realm must have a key server and audience")
	}

	return kss.ErrorOrNil()
}

// BeforeSave runs validations. If there are errors, the save fails.
func (kssd *KeyServerStatsDay) BeforeSave(tx *gorm.DB) error {
	if kssd.RealmID == 0 {
		kssd.AddError("realm_id", "statistics may not be saved on the system realm")
	}

	return kssd.ErrorOrNil()
}

// GetKeyServerStats retrieves the configuration for gathering key-server statistics
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

// SaveKeyServerStats stores the configuration for gathering key-server statistics
func (db *Database) SaveKeyServerStats(stats *KeyServerStats) error {
	return db.db.Save(stats).Error
}

// ListKeyServerStatsDays retrieves the last 30 days of key-server statistics
func (db *Database) ListKeyServerStatsDays(realmID uint, day time.Time) ([]*KeyServerStatsDay, error) {
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	var stats []*KeyServerStatsDay
	if err := db.db.
		Model(&KeyServerStatsDay{}).
		Where("realm_id = ? AND day >= ?", realmID, thirtyDaysAgo).
		Order("day DESC").
		Limit(30).
		Find(&stats).
		Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// SaveKeyServerStatsDay stores a single day of key-server statistics
func (db *Database) SaveKeyServerStatsDay(day *KeyServerStatsDay) error {
	return db.db.Debug().Save(day).Error
}
