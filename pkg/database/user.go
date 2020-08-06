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
	Email           string `gorm:"type:varchar(250);unique_index"`
	Name            string `gorm:"type:varchar(100)"`
	Admin           bool   `gorm:"default:false"`
	LastRevokeCheck time.Time
	Realms          []*Realm `gorm:"many2many:user_realms"`
	AdminRealms     []*Realm `gorm:"many2many:admin_realms"`
}

func (u *User) MultipleRealms() bool {
	return len(u.Realms) > 1
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

// ListUsers retrieves all of the configured users.
// Done without pagination.
// This is not scoped to realms.
func (db *Database) ListUsers(includeDeleted bool) ([]*User, error) {
	var users []*User

	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Order("email ASC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	return users, nil
}

// GetUser reads back a User struct by email address.
func (db *Database) GetUser(id uint) (*User, error) {
	var user User
	if err := db.db.Preload("Realms").Preload("AdminRealms").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUser reads back a User struct by email address.
func (db *Database) FindUser(email string) (*User, error) {
	var user User
	if err := db.db.Preload("Realms").Preload("AdminRealms").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a user record.
func (db *Database) CreateUser(email string, name string, admin bool) (*User, error) {
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("provided email address may not be valid, double check: '%v'", email)
	}

	if name == "" {
		name = parts[0]
	}

	user, err := db.FindUser(email)
	if err == gorm.ErrRecordNotFound {
		// New record.
		user = &User{}
	} else if err != nil {
		return nil, err
	}

	// Update fields
	user.Email = email
	user.Name = name
	user.Admin = admin

	if err := db.SaveUser(user); err != nil {
		return nil, err
	}

	return user, nil
}

// SaveUser creates or updates a user record.
func (db *Database) SaveUser(u *User) error {
	if u.Model.ID == 0 {
		return db.db.Create(u).Error
	}
	return db.db.Save(u).Error
}

// DeleteUser removes a user record.
func (db *Database) DeleteUser(u *User) error {
	return db.db.Delete(u).Error
}
