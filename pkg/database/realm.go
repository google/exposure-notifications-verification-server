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
	gorm.Model
	Errorable

	// Name is the name of the realm.
	Name string `gorm:"type:varchar(200);unique_index"`

	// These are here for gorm to setup the association. You should NOT call them
	// directly, ever. Use the ListUsers function instead. The have to be public
	// for reflection.
	RealmUsers  []*User `gorm:"many2many:user_realms;PRELOAD:false"`
	RealmAdmins []*User `gorm:"many2many:admin_realms;PRELOAD:false"`

	// Relations to items that blong to a realm.
	Codes  []*VerificationCode
	Tokens []*Token
}

// SMSConfig returns the SMS configuration for this realm, if one exists.
func (r *Realm) SMSConfig(db *Database) (*SMSConfig, error) {
	var smsConfig SMSConfig
	if err := db.db.
		Model(r).
		Related(&smsConfig, "SMSConfig").
		Error; err != nil {
		return nil, err
	}
	return &smsConfig, nil
}

// HasSMSConfig returns true if the realm has an SMS config, false otherwise.
// This does not perform the KMS encryption/decryption, so it's more efficient
// that loading the full SMS config.
func (r *Realm) HasSMSConfig(db *Database) (bool, error) {
	var smsConfig SMSConfig
	if err := db.db.
		Select("id").
		Model(r).
		Related(&smsConfig, "SMSConfig").
		Error; err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SMSProvider returns the SMS provider for the realm. If no sms configuration
// exists, it returns nil. If any errors occur creating the provider, they are
// returned.
func (r *Realm) SMSProvider(db *Database) (sms.Provider, error) {
	smsConfig, err := r.SMSConfig(db)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return smsConfig.SMSProvider(db)
}

// ListAuthorizedApps gets all the authorized apps for the realm.
func (r *Realm) ListAuthorizedApps(db *Database) ([]*AuthorizedApp, error) {
	var authApps []*AuthorizedApp
	if err := db.db.
		Unscoped().
		Model(r).
		Order("authorized_apps.deleted_at DESC, LOWER(authorized_apps.name)").
		Related(&authApps).
		Error; err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return authApps, nil
}

// FindAuthorizedApp finds the authorized app by the given id associated to the
// realm.
func (r *Realm) FindAuthorizedApp(db *Database, id interface{}) (*AuthorizedApp, error) {
	var app AuthorizedApp
	if err := db.db.
		Unscoped().
		Model(AuthorizedApp{}).
		Where("id = ? AND realm_id = ?", id, r.ID).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListUsers returns the list of users on this realm.
func (r *Realm) ListUsers(db *Database) ([]*User, error) {
	var users []*User
	if err := db.db.
		Model(r).
		Related(&users, "RealmUsers").
		Order("email").
		Error; err != nil {
		return nil, err
	}
	return users, nil
}

// FindUser finds the given user in the realm by ID.
func (r *Realm) FindUser(db *Database, id interface{}) (*User, error) {
	var user User
	if err := db.db.
		Table("users").
		Joins("INNER JOIN user_realms ON user_id = ? AND realm_id = ?", id, r.ID).
		Find(&user, "users.id = ?", id).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *Database) CreateRealm(name string) (*Realm, error) {
	var realm Realm
	realm.Name = name

	if err := db.db.Create(&realm).Error; err != nil {
		return nil, fmt.Errorf("unable to save realm: %w", err)
	}
	return &realm, nil
}

func (db *Database) GetRealmByName(name string) (*Realm, error) {
	var realm Realm

	if err := db.db.Where("name = ?", name).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) GetRealm(realmID uint) (*Realm, error) {
	var realm Realm

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
	if r.Model.ID == 0 {
		return db.db.Create(r).Error
	}
	return db.db.Save(r).Error
}
