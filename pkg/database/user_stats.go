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

// UserStats represents statistics related to a user in the database.
type UserStats struct {
	gorm.Model
	Date        time.Time `gorm:"unique_index:idx_date_user_realm"`
	UserID      uint      `gorm:"unique_index:idx_date_user_realm"`
	RealmID     uint      `gorm:"unique_index:idx_date_user_realm"`
	CodesIssued uint64
}

type UserStatsSummary struct {
	User           *User
	Realm          *Realm
	CodesIssued1d  uint64
	CodesIssued7d  uint64
	CodesIssued30d uint64
}

// TableName sets the UserStats table name
func (UserStats) TableName() string {
	return "user_stats"
}

func (db *Database) GetUserStatsSummary(u *User, r *Realm) (*UserStatsSummary, error) {
	t := time.Now().UTC()
	roundedTime := t.Truncate(24 * time.Hour)

	var summary = &UserStatsSummary{}
	var dailyStats []*UserStats

	// get the last 30 days of dates and counts for a given user in a realm.
	if err := db.db.
		Where("user_id = ?", u.ID).
		Where("realm_id = ?", r.ID).
		Where("date BETWEEN ? AND ?", roundedTime.AddDate(0, 0, -30), roundedTime).
		Find(&dailyStats).Error; err != nil {
		return nil, err
	}

	for _, userStats := range dailyStats {
		// All entires are 30d
		summary.CodesIssued30d += userStats.CodesIssued

		// Only one entry is 1d
		if userStats.Date == roundedTime {
			summary.CodesIssued1d += userStats.CodesIssued
		}

		// Find 7d entries
		if userStats.Date.After(roundedTime.AddDate(0, 0, -7)) {
			summary.CodesIssued7d += userStats.CodesIssued

		}
	}

	// create 24h, 7d, 30d counts
	return summary, nil
}

func (db *Database) GetUserStats(u *User, r *Realm, t time.Time) (*UserStats, error) {
	var userStats UserStats
	roundedTime := t.Truncate(24 * time.Hour)

	if err := db.db.
		Where("user_id = ?", u.ID).
		Where("realm_id = ?", r.ID).
		Where("date = ?", roundedTime).
		First(&userStats).Error; err != nil {
		return nil, err
	}
	return &userStats, nil
}

func (db *Database) UpdateUserStats() error {
	var userStats UserStats
	err := db.db.Order("date DESC").First(&userStats).Error

	if err != nil {
		return err
	}

	// Start from last day in authorized app table.
	curDay := userStats.Date
	t := time.Now().UTC()
	now := t.Truncate(24 * time.Hour)

	for {
		if curDay.After(now) {
			return nil
		}

		err := db.updateUserStatsDay(curDay)
		if err != nil {
			return err
		}
		curDay = curDay.AddDate(0, 0, 1)
	}

}

func (db *Database) updateUserStatsDay(t time.Time) error {
	roundedTime := t.Truncate(24 * time.Hour)

	// For each realm, and each user in the realm, gather and store stats
	realms, err := db.GetRealms()
	if err != nil {
		return err
	}

	for _, realm := range realms {
		realmUsers, err := realm.ListUsers(db)
		if err != nil {
			return err
		}

		for _, user := range realmUsers {
			codesIssued, err := db.countVerificationCodesByUser(realm.ID, user.ID, roundedTime)
			if err != nil {
				return err
			}

			var us UserStats
			err = db.db.
				Where("user_id = ?", user.ID).
				Where("realm_id = ?", realm.ID).
				Where("date = ?", roundedTime).
				First(&us).Error

			if err == gorm.ErrRecordNotFound {
				// New record.
				us = UserStats{}
			} else if err != nil {
				return err
			}

			us.Date = roundedTime
			us.RealmID = realm.ID
			us.UserID = user.ID
			us.CodesIssued = codesIssued

			if us.Model.ID == 0 {
				if err := db.db.Create(&us).Error; err != nil {
					return err
				}
			} else {
				if err := db.db.Save(&us).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (db *Database) countVerificationCodesByUser(realm uint, user uint, t time.Time) (uint64, error) {
	if user <= 0 {
		return 0, nil
	}

	// TODO: count operations require a table lock. Can this be done without?
	//var count uint64
	var rows []*VerificationCode
	if err := db.db.Model(&VerificationCode{}).
		Where("realm_id = ?", realm).
		Where("issuing_user_id = ?", user).
		Where("date_trunc('day', date(created_at)) = ?", t).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	return uint64(len(rows)), nil
}
