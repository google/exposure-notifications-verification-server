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
	"context"
	"strconv"
	"time"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"

	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
)

// KeyServerStats represents statistics for a key-server for this realm
type KeyServerStats struct {
	Errorable

	// RealmId that these stats belong to.
	RealmID uint `gorm:"column:realm_id; primary_key; type:integer; not null;"`

	// KeyServerURLOverride allows a realm to override the system's URL with its own
	KeyServerURLOverride string `gorm:"column:key_server_url_override; type:text;"`
	// KeyServerAudience allows a realm to override the system's audience
	KeyServerAudienceOverride string `gorm:"column:key_server_audience_override; type:text;"`
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
	if kss.RealmID == 0 && (kss.KeyServerURLOverride == "" || kss.KeyServerAudienceOverride == "") {
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

// TotalPublishRequests returns the sum of all publish requests for this
// day, which are stored by operating system.
func (kssd *KeyServerStatsDay) TotalPublishRequests() int64 {
	var sum int64
	for _, v := range kssd.PublishRequests {
		sum += v
	}
	return sum
}

// GetKeyServerStatsCached returns true if the provided realm has key
// server stats enabled.
func (db *Database) GetKeyServerStatsCached(ctx context.Context, realmID uint, cacher cache.Cacher) (*KeyServerStats, error) {
	var kss *KeyServerStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:key_server_enabled",
		Key:       strconv.FormatUint(uint64(realmID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &kss, 30*time.Minute, func() (interface{}, error) {
		val, err := db.GetKeyServerStats(realmID)
		if err != nil {
			if IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return val, nil
	}); err != nil {
		return nil, err
	}
	return kss, nil
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

// DeleteKeyServerStats disables gathering key-server statistics and removes the entry
func (db *Database) DeleteKeyServerStats(realmID uint) error {
	kss := &KeyServerStats{
		RealmID: realmID,
	}
	return db.db.Unscoped().
		Set("gorm:delete_option", "RETURNING *").
		Delete(kss).
		Error
}

// ListKeyServerStats retrieves the key-server statistics configuration for all realms
func (db *Database) ListKeyServerStats() ([]*KeyServerStats, error) {
	var stats []*KeyServerStats
	if err := db.db.
		Model(&KeyServerStats{}).
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

// DeleteOldKeyServerStatsDays deletes rows from KeyServerStatsDays that are older than maxAge (default 90d)
func (db *Database) DeleteOldKeyServerStatsDays(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	a := time.Now().UTC().Add(maxAge)
	rtn := db.db.Unscoped().
		Where("day < ?", a).
		Delete(&KeyServerStatsDay{})
	return rtn.RowsAffected, rtn.Error
}

// ListKeyServerStatsDaysCached retrieves the last 90 days of key-server statistics
func (db *Database) ListKeyServerStatsDaysCached(ctx context.Context, realmID uint, cacher cache.Cacher) ([]*KeyServerStatsDay, error) {
	var stats []*KeyServerStatsDay
	cacheKey := &cache.Key{
		Namespace: "stats:realm:key_server",
		Key:       strconv.FormatUint(uint64(realmID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return db.ListKeyServerStatsDays(realmID)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// ListKeyServerStatsDays retrieves the last 90 days of key-server statistics
func (db *Database) ListKeyServerStatsDays(realmID uint) ([]*KeyServerStatsDay, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	sql := `
		SELECT
			d.day AS day,
			$1 AS realm_id,
			COALESCE(s.publish_requests, array[0,0,0]::bigint[]) AS publish_requests,
			COALESCE(s.total_teks_published, 0) AS total_teks_published,
			COALESCE(s.revision_requests, 0) AS revision_requests,
			COALESCE(s.tek_age_distribution, array[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]::bigint[]) AS tek_age_distribution,
			COALESCE(s.onset_to_upload_distribution, array[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]::bigint[]) AS onset_to_upload_distribution,
			COALESCE(s.request_missing_onset_date, 0) AS request_missing_onset_date
		FROM (
			SELECT day::date FROM generate_series($2, $3, '1 day'::interval) day
		) d
		LEFT JOIN key_server_stats_days s ON s.realm_id = $1 AND s.day = d.day
		ORDER BY day DESC`

	var stats []*KeyServerStatsDay
	if err := db.db.Raw(sql, realmID, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// MakeKeyServerStatsDay creates a storage struct from a key-server StatsDay response
func MakeKeyServerStatsDay(realmID uint, d *keyserver.StatsDay) *KeyServerStatsDay {
	pr := make([]int64, 3)
	pr[OSTypeUnknown] = d.PublishRequests.UnknownPlatform
	pr[OSTypeIOS] = d.PublishRequests.IOS
	pr[OSTypeAndroid] = d.PublishRequests.Android

	return &KeyServerStatsDay{
		RealmID:                   realmID,
		Day:                       d.Day,
		PublishRequests:           pr,
		TotalTEKsPublished:        d.TotalTEKsPublished,
		RevisionRequests:          d.RevisionRequests,
		TEKAgeDistribution:        d.TEKAgeDistribution,
		OnsetToUploadDistribution: d.OnsetToUploadDistribution,
		RequestsMissingOnsetDate:  d.RequestsMissingOnsetDate,
	}
}

// ToResponse makes a json-marshallable StatsDay from a KetServerStatsDay
func (kssd *KeyServerStatsDay) ToResponse() *keyserver.StatsDay {
	reqs := keyserver.PublishRequests{}
	if l := len(kssd.PublishRequests); l == 3 {
		reqs.UnknownPlatform = kssd.PublishRequests[OSTypeUnknown]
		reqs.IOS = kssd.PublishRequests[OSTypeIOS]
		reqs.Android = kssd.PublishRequests[OSTypeAndroid]
	}

	return &keyserver.StatsDay{
		Day:                       kssd.Day,
		PublishRequests:           reqs,
		TotalTEKsPublished:        kssd.TotalTEKsPublished,
		RevisionRequests:          kssd.RevisionRequests,
		TEKAgeDistribution:        kssd.TEKAgeDistribution,
		OnsetToUploadDistribution: kssd.OnsetToUploadDistribution,
		RequestsMissingOnsetDate:  kssd.RequestsMissingOnsetDate,
	}
}
