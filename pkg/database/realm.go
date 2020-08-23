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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/jinzhu/gorm"
)

// TestType is a test type in the database.
type TestType int16

var (
	ErrNoSigningKeyManagement = errors.New("no signing key management")
)

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
	RegionCode       string          `gorm:"type:varchar(10); not null; default: ''"`
	CodeLength       uint            `gorm:"type:smallint; not null; default: 8"`
	CodeDuration     DurationSeconds `gorm:"type:bigint; not null; default: 900"` // default 15m (in seconds)
	LongCodeLength   uint            `gorm:"type:smallint; not null; default: 16"`
	LongCodeDuration DurationSeconds `gorm:"type:bigint; not null; default: 86400"` // default 24h
	// SMS Content
	SMSTextTemplate string `gorm:"type:varchar(400); not null; default: 'This is your Exposure Notifications Verification code: ens://v?r=[region]&c=[longcode] Expires in [longexpires] hours'"`

	// AllowedTestTypes is the type of tests that this realm permits. The default
	// value is to allow all test types.
	AllowedTestTypes TestType `gorm:"type:smallint; not null; default: 14"`

	// Signing Key Settings
	UseRealmCertificateKey bool            `gorm:"type:boolean; default: false"`
	CertificateIssuer      string          `gorm:"type:varchar(150); default ''"`
	CertificateAudience    string          `gorm:"type:varchar(150); default ''"`
	CertificateDuration    DurationSeconds `gorm:"type:bigint; default: 900"` // 15m

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

func (r *Realm) CanUpgradeToRealmSigningKeys() bool {
	return r.CertificateIssuer != "" && r.CertificateAudience != ""
}

func (r *Realm) SigningKeyID() string {
	return fmt.Sprintf("realm-%d", r.ID)
}

func (r *Realm) EnsureSigningKeyExists(ctx context.Context, db *Database, keyRing string) error {
	if db.signingKeyManager == nil {
		return ErrNoSigningKeyManagement
	}

	logger := logging.FromContext(ctx)
	// Ensure the realm has a signing key.
	realmKeys, err := r.ListSigningKeys(db)
	if err != nil {
		return fmt.Errorf("unable to list signing keys for realm: %w", err)
	}
	if len(realmKeys) > 0 {
		return nil
	}

	versions, err := db.signingKeyManager.SigningKeyVersions(ctx, keyRing, r.SigningKeyID())
	if err != nil {
		return fmt.Errorf("unable to list signing keys on kms: %w", err)
	}
	for _, v := range versions {
		if v.DestroyedAt().IsZero() {
			return nil
		}
	}

	id, err := db.signingKeyManager.CreateSigningKeyVersion(ctx, keyRing, r.SigningKeyID())
	if err != nil {
		return fmt.Errorf("unable to create signing key for realm: %w", err)
	}
	logger.Infow("provisioned certificate signing key for realm", "keyID", id)
	return nil
}

// BeforeSave runs validations. If there are errors, the save fails.
func (r *Realm) BeforeSave(tx *gorm.DB) error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		r.AddError("name", "cannot be blank")
	}

	r.RegionCode = strings.ToUpper(strings.TrimSpace(r.RegionCode))

	if len(r.RegionCode) > 10 {
		r.AddError("regionCode", "cannot be more than 10 characters")
	}

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

	ctx := context.Background()
	provider, err := sms.ProviderFor(ctx, &sms.Config{
		ProviderType:     smsConfig.ProviderType,
		TwilioAccountSid: smsConfig.TwilioAccountSid,
		TwilioAuthToken:  smsConfig.TwilioAuthToken,
		TwilioFromNumber: smsConfig.TwilioFromNumber,
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (r *Realm) GetCurrentSigningKey(db *Database) (*SigningKey, error) {
	var signingKey SigningKey
	if err := db.db.
		Model(r).
		Order("signing_keys.created_at DESC").
		First(&signingKey).
		Error; err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to find signing key: %w", err)
	}
	return &signingKey, nil
}

func (r *Realm) SetActiveSigningKey(db *Database, keyID uint) (string, error) {
	var kid string
	err := db.db.Transaction(func(tx *gorm.DB) error {
		var keys []*SigningKey
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("realm_id = ?", r.Model.ID).
			Find(&keys).
			Error; err != nil {
			return err
		}

		found := false
		for _, k := range keys {
			if k.Model.ID == keyID {
				k.Active = true
				found = true
				if err := tx.Save(k).Error; err != nil {
					return err
				}
				kid = k.GetKID()
			} else {
				if k.Active {
					k.Active = false
					if err := tx.Save(k).Error; err != nil {
						return err
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("key to activate was not found")
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	return kid, nil
}

func (r *Realm) ListSigningKeys(db *Database) ([]*SigningKey, error) {
	var keys []*SigningKey
	if err := db.db.
		Model(r).
		Order("signing_keys.created_at DESC").
		Related(&keys).
		Error; err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return keys, nil
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

func (r *Realm) CreateNewSigningKeyVersion(ctx context.Context, db *Database) (string, error) {
	if db.signingKeyManager == nil {
		return "", ErrNoSigningKeyManager
	}

	keyRing := db.config.CertificateSigningKeyRing

	id, err := db.signingKeyManager.CreateSigningKeyVersion(ctx, keyRing, r.SigningKeyID())
	if err != nil {
		return "", fmt.Errorf("unable to create signing key for realm: %w", err)
	}
	db.logger.Infow("provisioned certificate signing key for realm", "keyID", id)

	curKeys, err := r.ListSigningKeys(db)
	if err != nil {
		return "", fmt.Errorf("unable to list existing signing keys: %w", err)
	}

	// Save this SigningKey record
	signingKey := SigningKey{
		RealmID: r.Model.ID,
		KeyID:   id,
		Active:  len(curKeys) == 0,
	}
	if err := db.SaveSigningKey(&signingKey); err != nil {
		return "", fmt.Errorf("failed to save reference to signing key: %w", err)
	}

	return signingKey.GetKID(), nil
}
