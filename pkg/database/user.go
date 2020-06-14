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

// User represents a user of the system
type User struct {
	gorm.Model
	Email           string `gorm:"type:varchar(250);unique_index"`
	Name            string `gorm:"type:varchar(100)"`
	Admin           bool   `gorm:"default:false"`
	Disabled        bool
	LastRevokeCheck time.Time
}

// FindUser reads back a User struct by email address.
func (db *Database) FindUser(email string) (*User, error) {
	var user User
	if err := db.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// SaveUser creates or updates a user record.
func (db *Database) SaveUser(u *User) error {
	if u.Model.ID == 0 {
		return db.db.Create(u).Error
	}
	return db.db.Save(u).Error
}
