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
	"time"

	"github.com/jinzhu/gorm"
)

// AuthorizedAppStats represents statistics related to a user in the database.
type AuthorizedAppStats struct {
	gorm.Model
	Date            time.Time `gorm:"unique_index:idx_date_app_realm"`
	AuthorizedAppID uint      `gorm:"unique_index:idx_date_app_realm"`
	RealmID         uint      `gorm:"unique_index:idx_date_app_realm"`
	CodesIssued     uint64
}

type AuthorizedAppStatsSummary struct {
	AuthorizedApp  *AuthorizedApp
	Realm          *Realm
	CodesIssued1d  uint64
	CodesIssued7d  uint64
	CodesIssued30d uint64
}

// TableName sets the AuthorizedAppStats table name
func (AuthorizedAppStats) TableName() string {
	return "authorized_app_stats"
}

func (db *Database) GetAuthorizedAppStatsSummary(a *AuthorizedApp, r *Realm) (*AuthorizedAppStatsSummary, error) {
	t := time.Now().UTC()
	roundedTime := t.Truncate(24 * time.Hour)

	var summary = &AuthorizedAppStatsSummary{}
	var dailyStats []*AuthorizedAppStats

	// get the last 30 days of dates and counts for a given user in a realm.
	if err := db.db.
		Where("authorized_app_id = ?", a.ID).
		Where("realm_id = ?", r.ID).
		Where("date BETWEEN ? AND ?", roundedTime.AddDate(0, 0, -30), roundedTime).
		Find(&dailyStats).Error; err != nil {
		return nil, err
	}

	for _, AuthorizedAppStats := range dailyStats {
		// All entires are 30d
		summary.CodesIssued30d += AuthorizedAppStats.CodesIssued

		// Only one entry is 1d
		if AuthorizedAppStats.Date == roundedTime {
			summary.CodesIssued1d += AuthorizedAppStats.CodesIssued
		}

		// Find 7d entries
		if AuthorizedAppStats.Date.After(roundedTime.AddDate(0, 0, -7)) {
			summary.CodesIssued7d += AuthorizedAppStats.CodesIssued

		}
	}

	// create 24h, 7d, 30d counts
	return summary, nil
}

func (db *Database) GetAuthorizedAppStats(a *AuthorizedApp, r *Realm, t time.Time) (*AuthorizedAppStats, error) {
	var AuthorizedAppStats AuthorizedAppStats
	roundedTime := t.Truncate(24 * time.Hour)

	if err := db.db.
		Where("authorized_app_id = ?", a.ID).
		Where("realm_id = ?", r.ID).
		Where("date = ?", roundedTime).
		First(&AuthorizedAppStats).Error; err != nil {
		return nil, err
	}
	return &AuthorizedAppStats, nil
}

func (db *Database) UpdateAuthorizedAppStats() error {
	var appStats AuthorizedAppStats
	err := db.db.Order("date DESC").First(&appStats).Error

	if err != nil {
		return err
	}

	// Start from last day in authorized app table.
	curDay := appStats.Date
	t := time.Now().UTC()
	now := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)

	for {
		if curDay.After(now) {
			return nil
		}

		err := db.updateAuthorizedAppStatsDay(curDay)
		if err != nil {
			return err
		}
		curDay = curDay.AddDate(0, 0, 1)
	}

}

func (db *Database) updateAuthorizedAppStatsDay(t time.Time) error {
	roundedTime := t.Truncate(24 * time.Hour)

	// For each realm, and each user in the realm, gather and store stats
	realms, err := db.GetRealms()
	if err != nil {
		return err
	}

	for _, realm := range realms {
		apps, err := realm.ListAuthorizedApps(db)
		if err != nil {
			return err
		}

		for _, app := range apps {
			codesIssued, err := db.countVerificationCodesByAuthorizedApp(realm.ID, app.ID, roundedTime)
			if err != nil {
				return err
			}

			var appStats AuthorizedAppStats
			err = db.db.
				Where("authorized_app_id = ?", app.ID).
				Where("realm_id = ?", realm.ID).
				Where("date = ?", roundedTime).
				First(&appStats).Error

			if err == gorm.ErrRecordNotFound {
				// New record.
				appStats = AuthorizedAppStats{}
			} else if err != nil {
				return err
			}

			appStats.Date = roundedTime
			appStats.RealmID = realm.ID
			appStats.AuthorizedAppID = app.ID
			appStats.CodesIssued = codesIssued

			if appStats.Model.ID == 0 {
				if err := db.db.Create(&appStats).Error; err != nil {
					return err
				}
			} else {
				if err := db.db.Save(&appStats).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (db *Database) countVerificationCodesByAuthorizedApp(realmID uint, appID uint, t time.Time) (uint64, error) {
	if appID <= 0 {
		return 0, nil
	}

	var rows []*VerificationCode
	if err := db.db.Model(&VerificationCode{}).
		Where("realm_id = ?", realmID).
		Where("issuing_app_id = ?", appID).
		Where("date_trunc('day', date(created_at)) = ?", t).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	return uint64(len(rows)), nil
}
