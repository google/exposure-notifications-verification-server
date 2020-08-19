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
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

// User represents a user of the system
type User struct {
	gorm.Model
	Errorable

	Email           string `gorm:"type:varchar(250);unique_index"`
	Name            string `gorm:"type:varchar(100)"`
	Admin           bool   `gorm:"default:false"`
	LastRevokeCheck time.Time
	Realms          []*Realm `gorm:"many2many:user_realms"`
	AdminRealms     []*Realm `gorm:"many2many:admin_realms"`
}

// BeforeSave runs validations. If there are errors, the save fails.
func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Email = strings.TrimSpace(u.Email)
	u.Name = strings.TrimSpace(u.Name)

	if u.Email == "" {
		u.AddError("email", "cannot be blank")
	}

	if !strings.Contains(u.Email, "@") {
		u.AddError("email", "appears to be invalid")
	}

	if u.Name == "" {
		u.AddError("name", "cannot be blank")
	}

	if len(u.Errors()) > 0 {
		return fmt.Errorf("validation failed")
	}
	return nil
}

func (u *User) GetRealm(realmID uint) *Realm {
	for _, r := range u.Realms {
		if r.ID == realmID {
			return r
		}
	}
	return nil
}

func (u *User) CanViewRealm(realmID uint) bool {
	for _, r := range u.Realms {
		if r.ID == realmID {
			return true
		}
	}
	return false
}

func (u *User) CanAdminRealm(realmID uint) bool {
	for _, r := range u.AdminRealms {
		if r.ID == realmID {
			return true
		}
	}
	return false
}

// AddRealm adds the user to the realm.
func (u *User) AddRealm(realm *Realm) {
	u.Realms = append(u.Realms, realm)
}

// AddRealmAdmin adds the user to the realm as an admin.
func (u *User) AddRealmAdmin(realm *Realm) {
	u.AdminRealms = append(u.AdminRealms, realm)
	u.AddRealm(realm)
}

// RemoveRealm removes the user from the realm. It also removes the user as an
// admin of that realm. You must save the user to persist the changes.
func (u *User) RemoveRealm(realm *Realm) {
	for i, r := range u.Realms {
		if r.ID == realm.ID {
			u.Realms = append(u.Realms[:i], u.Realms[i+1:]...)
		}
	}
	u.RemoveRealmAdmin(realm)
}

// RemoveRealmAdmin removes the user from the realm. You must save the user to
// persist the changes.
func (u *User) RemoveRealmAdmin(realm *Realm) {
	for i, r := range u.AdminRealms {
		if r.ID == realm.ID {
			u.AdminRealms = append(u.AdminRealms[:i], u.AdminRealms[i+1:]...)
		}
	}
}

// FindUser finds a user by the given id, if one exists. The id can be a string
// or integer value. It returns an error if the record is not found.
func (db *Database) FindUser(id interface{}) (*User, error) {
	var user User
	if err := db.db.
		Where("id = ?", id).
		First(&user).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByEmail reads back a User struct by email address. It returns an
// error if the record is not found.
func (db *Database) FindUserByEmail(email string) (*User, error) {
	var user User
	if err := db.db.
		Where("email = ?", email).
		First(&user).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Stats returns the usage statistics for this user at the provided realm. If no
// stats exist, it returns an empty array.
func (u *User) Stats(db *Database, realmID uint, start, stop time.Time) ([]*UserStats, error) {
	var stats []*UserStats

	start = start.Truncate(24 * time.Hour)
	stop = stop.Truncate(24 * time.Hour)

	if err := db.db.
		Model(&UserStats{}).
		Where("user_id = ?", u.ID).
		Where("realm_id = ?", realmID).
		Where("date >= ? AND date <= ?", start, stop).
		Order("date DESC").
		Find(&stats).
		Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}

	return stats, nil
}

// SaveUser updates the user in the database.
func (db *Database) SaveUser(u *User) error {
	db.db.Model(u).Association("Realms").Replace(u.Realms)
	db.db.Model(u).Association("AdminRealms").Replace(u.AdminRealms)
	return db.db.Save(u).Error
}
