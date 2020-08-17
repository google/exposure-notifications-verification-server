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

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm"
)

// TestType is a test type in the database.
type TestType int16

const (
	_ TestType = 1 << iota
	TestTypeConfirmed
	TestTypeLikely
	TestTypeNegative
)

const (
	maxCodeDuration     = time.Hour
	maxLongCodeDuration = 24 * time.Hour

	SMSRegion      = "[region]"
	SMSCode        = "[code]"
	SMSExpires     = "[expires]"
	SMSLongCode    = "[longcode]"
	SMSLongExpires = "[longexpires]"
)

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	gorm.Model
	Errorable

	// Name is the name of the realm.
	Name string `gorm:"type:varchar(200);unique_index"`

	// Code configuration
	RegionCode       string          `gorm:"type:varchar(5); not null; default: ''"`
	CodeLength       uint            `gorm:"type:smallint; not null; default: 8"`
	CodeDuration     DurationSeconds `gorm:"type:bigint; not null; default: 900"` // default 15m (in seconds)
	LongCodeLength   uint            `gorm:"type:smallint; not null; default: 16"`
	LongCodeDuration DurationSeconds `gorm:"type:bigint; not null; default: 86400"` // default 24h
	// SMS Content
	SMSTextTemplate string `gorm:"type:varchar(400); not null; default: 'This is your Exposure Notifications Verification code: ens://v?r=[region]&c=[longcode] Expires in [longexpires] hours'"`

	// AllowedTestTypes is the type of tests that this realm permits. The default
	// value is to allow all test types.
	AllowedTestTypes TestType `gorm:"type:smallint; not null; default: 14"`

	// These are here for gorm to setup the association. You should NOT call them
	// directly, ever. Use the ListUsers function instead. The have to be public
	// for reflection.
	RealmUsers  []*User `gorm:"many2many:user_realms; PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
	RealmAdmins []*User `gorm:"many2many:admin_realms; PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`

	// Relations to items that belong to a realm.
	Codes  []*VerificationCode `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
	Tokens []*Token            `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
}

// NewRealmWithDefaults initializes a new Realm with the default settings populated,
// and the provided name. It does NOT save the Realm to the database.
func NewRealmWithDefaults(name string) *Realm {
	return &Realm{
		Name:             name,
		CodeLength:       8,
		CodeDuration:     DurationSeconds{Duration: 15 * time.Minute},
		LongCodeLength:   16,
		LongCodeDuration: DurationSeconds{Duration: 24 * time.Hour},
		SMSTextTemplate:  "This is your Exposure Notifications Verification code: ens://v?r=[region]&c=[longcode] Expires in [longexpires] hours",
		AllowedTestTypes: 14,
	}
}

// BeforeSave runs validations. If there are errors, the save fails.
func (r *Realm) BeforeSave(tx *gorm.DB) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		r.AddError("name", "cannot be blank")
	}

	r.RegionCode = strings.ToUpper(strings.TrimSpace(r.RegionCode))

	if r.CodeLength < 6 {
		r.AddError("codeLength", "must be at least 6")
	}
	if r.CodeDuration.Duration > maxCodeDuration {
		r.AddError("codeDuration", "must be no more than 1 hour")
	}

	if r.LongCodeLength < 12 {
		r.AddError("longCodeLength", "must be at least 12")
	}
	if r.LongCodeDuration.Duration > maxLongCodeDuration {
		r.AddError("longCodeDuration", "must be no more than 24 hours")
	}

	// Check that we have exactly one of [code] or [longcode] as template substitutions.
	if c, lc := strings.Contains(r.SMSTextTemplate, "[code]"), strings.Contains(r.SMSTextTemplate, "[longcode]"); !(c || lc) || (c && lc) {
		r.AddError("SMSTextTemplate", "must contain exactly one of [code] or [longcode]")
	}

	if len(r.Errors()) > 0 {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// GetCodeDurationMinutes is a helper for the HTML rendering to get a round
// minutes value.
func (r *Realm) GetCodeDurationMinutes() int {
	return int(r.CodeDuration.Duration.Minutes())
}

// GetLongCodeDurationHours is a helper for the HTML rendering to get a round
// hours value.
func (r *Realm) GetLongCodeDurationHours() int {
	return int(r.LongCodeDuration.Duration.Hours())
}

// BuildSMSText replaces certain strings with the right values.
func (r *Realm) BuildSMSText(code, longCode string) string {
	text := r.SMSTextTemplate

	text = strings.ReplaceAll(text, SMSRegion, r.RegionCode)
	text = strings.ReplaceAll(text, SMSCode, code)
	text = strings.ReplaceAll(text, SMSExpires, fmt.Sprintf("%d", r.GetCodeDurationMinutes()))
	text = strings.ReplaceAll(text, SMSLongCode, longCode)
	text = strings.ReplaceAll(text, SMSLongExpires, fmt.Sprintf("%d", r.GetLongCodeDurationHours()))

	return text
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

// ValidTestType returns true if the given test type string is valid for this
// realm, false otherwise.
func (r *Realm) ValidTestType(typ string) bool {
	switch strings.TrimSpace(strings.ToLower(typ)) {
	case "confirmed":
		return r.AllowedTestTypes&TestTypeConfirmed != 0
	case "likely":
		return r.AllowedTestTypes&TestTypeLikely != 0
	case "negative":
		return r.AllowedTestTypes&TestTypeNegative != 0
	default:
		return false
	}
}

func (db *Database) CreateRealm(name string) (*Realm, error) {
	realm := NewRealmWithDefaults(name)

	if err := db.db.Create(realm).Error; err != nil {
		return nil, fmt.Errorf("unable to save realm: %w", err)
	}
	return realm, nil
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
