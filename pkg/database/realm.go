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

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	db *Database `gorm:"-"`

	gorm.Model

	Name string `gorm:"type:varchar(200);unique_index"`

	AuthorizedApps []*AuthorizedApp

	RealmUsers  []*User `gorm:"many2many:user_realms"`
	RealmAdmins []*User `gorm:"many2many:admin_realms"`

	// Relations to items that blong to a realm.
	Codes  []*VerificationCode
	Tokens []*Token
}

// SMSConfig returns the SMS config for the realm, if one exists.
func (r *Realm) SMSConfig() (*SMSConfig, error) {
	var c SMSConfig
	c.db = r.db

	if err := r.db.db.Model(r).Related(&c).Error; err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &c, nil
}

// HasSMSConfig returns true if the realm has SMS configuration, false
// otherwise.
func (r *Realm) HasSMSConfig() bool {
	c, _ := r.SMSConfig()
	return c != nil
}

// SMSProvider returns the SMS provider for the realm. If no sms configuration
// exists, it returns nil. If any errors occur creating the provider, they are
// returned.
func (r *Realm) SMSProvider() (sms.Provider, error) {
	c, err := r.SMSConfig()
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, nil
	}

	return c.SMSProvider()
}

// AddAuthorizedApp adds to the in memory structure, but does not save.
// Use SaveRealm to persist.
func (r *Realm) AddAuthorizedApp(a *AuthorizedApp) {
	r.AuthorizedApps = append(r.AuthorizedApps, a)
}

// AddUser add to the in memory structure, but does not save.
// Use SaveRealm to persist.
func (r *Realm) AddUser(u *User) {
	for _, cUser := range r.RealmUsers {
		if cUser.ID == u.ID {
			return
		}
	}
	r.RealmUsers = append(r.RealmUsers, u)
}

// AddAdminUser adds to the in memory structure, but does not save.
// Use SaveRealm to persist.
func (r *Realm) AddAdminUser(u *User) {
	// To be an admin of the realm you also have to be a user of the realm.
	r.AddUser(u)
	for _, cUser := range r.RealmAdmins {
		if cUser.ID == u.ID {
			return
		}
	}
	r.RealmAdmins = append(r.RealmAdmins, u)
}

// LoadRealmUsers performs a lazy load over the users of the realm.
// Really only needed for user admin scenarios.
func (r *Realm) LoadRealmUsers(db *Database, includeDeleted bool) error {
	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Model(r).Preload("Realms").Preload("AdminRealms").Order("email").Related(&r.RealmUsers, "RealmUsers").Error; err != nil {
		return fmt.Errorf("unable to load realm users: %w", err)
	}
	if err := scope.Model(r).Preload("Realms").Preload("AdminRealms").Order("email").Related(&r.RealmAdmins, "RealmAdmins").Error; err != nil {
		return fmt.Errorf("unable to load realm admins: %w", err)
	}

	return nil
}

// GetAuthorizedApps does a lazy load on a realm's authorized apps if they are not already loaded.
func (r *Realm) GetAuthorizedApps(db *Database, includeDeleted bool) ([]*AuthorizedApp, error) {
	if len(r.AuthorizedApps) > 0 {
		return r.AuthorizedApps, nil
	}
	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Model(r).Related(&r.AuthorizedApps).Error; err != nil {
		return nil, err
	}
	return r.AuthorizedApps, nil
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
			return fmt.Errorf("failed to check other user associations: %w", err)
		}
		if len(user.AdminRealms) == 0 && len(user.Realms) == 0 {
			if err := tx.Delete(user).Error; err != nil {
				return fmt.Errorf("unable to delete user: %w", err)
			}
		}
		return nil
	})
}

func (db *Database) CreateRealm(name string) (*Realm, error) {
	var realm Realm
	realm.db = db
	realm.Name = name

	if err := db.db.Create(&realm).Error; err != nil {
		return nil, fmt.Errorf("unable to save realm: %w", err)
	}
	return &realm, nil
}

func (db *Database) GetRealmByName(name string) (*Realm, error) {
	var realm Realm
	realm.db = db

	if err := db.db.Where("name = ?", name).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) GetRealm(realmID uint) (*Realm, error) {
	var realm Realm
	realm.db = db

	if err := db.db.Where("id = ?", realmID).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) GetRealms() ([]*Realm, error) {
	var realms []*Realm
	if err := db.db.Find(&realms).Error; err != nil {
		return nil, err
	}
	return realms, nil
}

func (db *Database) SaveRealm(r *Realm) error {
	r.db = db

	if r.Model.ID == 0 {
		return db.db.Create(r).Error
	}
	return db.db.Save(r).Error
}
