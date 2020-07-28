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

	"github.com/jinzhu/gorm"
)

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	gorm.Model
	Name string `gorm:"type:varchar(200);unique_index"`

	AuthorizedApps []*AuthorizedApp

	RealmUsers  []*User `gorm:"many2many:user_realms"`
	RealmAdmins []*User `gorm:"many2many:admin_realms"`

	// Relations to items that blong to a realm.
	Codes  []*VerificationCode
	Tokens []*Token
}

func (r *Realm) AddAuthorizedApp(a *AuthorizedApp) {
	r.AuthorizedApps = append(r.AuthorizedApps, a)
}

func (r *Realm) AddUser(u *User) {
	for _, cUser := range r.RealmUsers {
		if cUser.ID == u.ID {
			return
		}
	}
	r.RealmUsers = append(r.RealmUsers, u)
}

func (r *Realm) AddAdminUser(u *User) {
	for _, cUser := range r.RealmAdmins {
		if cUser.ID == u.ID {
			return
		}
	}
	r.RealmAdmins = append(r.RealmAdmins, u)
}

func (r *Realm) LoadRealmUsers(db *Database, includeDeleted bool) error {
	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Model(r).Related(&r.RealmUsers, "RealmUsers").Error; err != nil {
		return fmt.Errorf("unable to load realm users: %w", err)
	}
	if err := scope.Model(r).Related(&r.RealmAdmins, "RealmAdmins").Error; err != nil {
		return fmt.Errorf("unable to load realm admins: %w", err)
	}

	return nil
}

func (r *Realm) LoadAuthorizedApps(db *Database, includeDeleted bool) error {
	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	return scope.Model(r).Related(&r.AuthorizedApps).Error
}

func (r *Realm) DeleteUserFromRealm(db *Database, u *User) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(r).Association("RealmUsers").Delete(u).Error; err != nil {
			return fmt.Errorf("unable to remove user from realm: %w", err)
		}
		if err := tx.Model(r).Association("RealmAdmins").Delete(u).Error; err != nil {
			return fmt.Errorf("unable to remove user from realm admins: %w", err)
		}

		// If the user no has no associations, the user should be deleted.
		var user User
		if err := tx.Preload("Realms").Preload("AdminRealms").Where("id = ?", u.ID).First(&user).Error; err != nil {
			return err
		}
		if len(user.AdminRealms) == 0 && len(user.Realms) == 0 {
			if err := tx.Delete(user).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *Database) CreateRealm(name string) (*Realm, error) {
	realm := Realm{
		Name: name,
	}
	if err := db.db.Create(&realm).Error; err != nil {
		return nil, fmt.Errorf("unable to save realm: %w", err)
	}
	return &realm, nil
}

func (db *Database) GetRealm(realmID int64) (*Realm, error) {
	var realm Realm
	if err := db.db.Where("id = ?", realmID).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) SaveRealm(r *Realm) error {
	if r.Model.ID == 0 {
		return db.db.Create(r).Error
	}
	return db.db.Save(r).Error
}
