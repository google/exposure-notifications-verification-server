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

	SMSRegion        = "[region]"
	SMSCode          = "[code]"
	SMSExpires       = "[expires]"
	SMSLongCode      = "[longcode]"
	SMSLongExpires   = "[longexpires]"
	SMSENExpressLink = "[enslink]"
)

// MFAMode represents Multi Factor Authentication requirements for the realm
type MFAMode int16

const (
	// MFAOptionalPrompt will prompt users for MFA on login.
	MFAOptionalPrompt = iota
	// MFARequired will not allow users to proceed without MFA on their account.
	MFARequired
	// MFAOptional will not prompt users to enable MFA.
	MFAOptional
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

	MFAMode MFAMode `gorm:"type:smallint; not null; default: 0"`

	// AllowedTestTypes is the type of tests that this realm permits. The default
	// value is to allow all test types.
	AllowedTestTypes TestType `gorm:"type:smallint; not null; default: 14"`

	// Signing Key Settings
	UseRealmCertificateKey bool            `gorm:"type:boolean; default: false"`
	CertificateIssuer      string          `gorm:"type:varchar(150); default: ''"`
	CertificateAudience    string          `gorm:"type:varchar(150); default: ''"`
	CertificateDuration    DurationSeconds `gorm:"type:bigint; default: 900"` // 15m

	// EN Express
	EnableENExpress bool `gorm:"type:boolean; default: false"`

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
		Name:                name,
		CodeLength:          8,
		CodeDuration:        FromDuration(15 * time.Minute),
		LongCodeLength:      16,
		LongCodeDuration:    FromDuration(24 * time.Hour),
		SMSTextTemplate:     "This is your Exposure Notifications Verification code: ens://v?r=[region]&c=[longcode] Expires in [longexpires] hours",
		AllowedTestTypes:    14,
		CertificateDuration: FromDuration(15 * time.Minute),
	}
}

func (r *Realm) CanUpgradeToRealmSigningKeys() bool {
	return r.CertificateIssuer != "" && r.CertificateAudience != ""
}

func (r *Realm) SigningKeyID() string {
	return fmt.Sprintf("realm-%d", r.ID)
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

	if r.EnableENExpress {
		if r.RegionCode == "" {
			r.AddError("regionCode", "cannot be blank when using EN Express")
		} else {
			parts := strings.Split(r.RegionCode, "-")
			if len(parts) != 2 {
				r.AddError("regionCode", "must be formated like 'region-subregion', 2 characters dash 2 or 3 characters")
			} else {
				if len(parts[0]) != 2 {
					r.AddError("regionCode", "first part must be exactly 2 characters in length")
				}
				if l := len(parts[1]); !(l == 2 || l == 3) {
					r.AddError("regionCode", "second part must be exactly 2 or 3 characters in length")
				}
			}
		}
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

	if r.EnableENExpress {
		if !strings.Contains(r.SMSTextTemplate, SMSENExpressLink) {
			r.AddError("SMSTextTemplate", fmt.Sprintf("must contain %q", SMSENExpressLink))
		}
		if strings.Contains(r.SMSTextTemplate, SMSRegion) {
			r.AddError("SMSTextTemplate", fmt.Sprintf("cannot contain %q - this is automatically included in %q", SMSRegion, SMSENExpressLink))
		}
		if strings.Contains(r.SMSTextTemplate, SMSCode) {
			r.AddError("SMSTextTemplate", fmt.Sprintf("cannot contain %q - the long code is automatically included in %q", SMSCode, SMSENExpressLink))
		}
		if strings.Contains(r.SMSTextTemplate, SMSExpires) {
			r.AddError("SMSTextTemplate", fmt.Sprintf("cannot contain %q - only the %q is allwoed for expiration", SMSExpires, SMSLongExpires))
		}
		if strings.Contains(r.SMSTextTemplate, SMSLongCode) {
			r.AddError("SMSTextTemplate", fmt.Sprintf("cannot contain %q - the long code is automatically included in %q", SMSLongCode, SMSENExpressLink))
		}

	} else {
		// Check that we have exactly one of [code] or [longcode] as template substitutions.
		if c, lc := strings.Contains(r.SMSTextTemplate, "[code]"), strings.Contains(r.SMSTextTemplate, "[longcode]"); !(c || lc) || (c && lc) {
			r.AddError("SMSTextTemplate", "must contain exactly one of [code] or [longcode]")
		}
	}

	if r.UseRealmCertificateKey {
		if r.CertificateIssuer == "" {
			r.AddError("certificateIssuer", "cannot be blank")
		}
		if r.CertificateAudience == "" {
			r.AddError("certificateAudience", "cannot be blank")
		}
	}

	if r.CertificateDuration.AsString != "" {
		if err := r.CertificateDuration.Update(); err != nil {
			r.AddError("certificateDuration", "invalid certificate duration")
		}
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

	text = strings.ReplaceAll(text, SMSENExpressLink, fmt.Sprintf("ens://v?r=%s&c=%s", SMSRegion, SMSLongCode))
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

// GetCurrentSigningKey returns the currently active signing key, the one marked
// active in the database. If there is more than one active, the most recently
// created one wins. Should not occur due to transactional update.
func (r *Realm) GetCurrentSigningKey(db *Database) (*SigningKey, error) {
	var signingKey SigningKey
	if err := db.db.
		Where("realm_id = ?", r.ID).
		Where("active = ?", true).
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

// SetActiveSigningKey sets a specific signing key to active=true for the realm,
// and transactionally sets all other signing keys to inactive. It accepts the
// database primary key ID but returns the KID of the now-active key.
func (r *Realm) SetActiveSigningKey(db *Database, id uint) (string, error) {
	var signingKey SigningKey

	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Find the key that should be active - do this first to ensure that the
		// provided PK id is actually valid.
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Table("signing_keys").
			Where("id = ?", id).
			Where("realm_id = ?", r.ID).
			First(&signingKey).
			Error; err != nil {
			if IsNotFound(err) {
				return fmt.Errorf("key to activate does not exist")
			}
			return fmt.Errorf("failed to find newly active key: %w", err)
		}

		// Mark all other keys as inactive.
		if err := tx.
			Table("signing_keys").
			Where("realm_id = ?", r.ID).
			Where("id != ?", id).
			Update("active", false).
			Error; err != nil {
			return fmt.Errorf("failed to mark existing keys as inactive: %w", err)
		}

		// Mark the active key as active.
		signingKey.Active = true
		if err := tx.Save(&signingKey).Error; err != nil {
			return fmt.Errorf("failed to mark new key as active: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}

	return signingKey.GetKID(), nil
}

// ListSigningKeys returns the non-deleted signing keys for a realm
// ordered by created_at desc.
func (r *Realm) ListSigningKeys(db *Database) ([]*SigningKey, error) {
	var keys []*SigningKey
	if err := db.db.
		Model(r).
		Order("signing_keys.created_at DESC").
		Related(&keys).
		Error; err != nil {
		if IsNotFound(err) {
			return keys, nil
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
		Order("LOWER(name)").
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
		Order("LOWER(name)").
		Related(&users, "RealmUsers").
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

func (db *Database) FindRealmByName(name string) (*Realm, error) {
	var realm Realm

	if err := db.db.Where("name = ?", name).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) FindRealm(id interface{}) (*Realm, error) {
	var realm Realm
	if err := db.db.
		Where("id = ?", id).
		First(&realm).
		Error; err != nil {
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
	return db.db.Save(r).Error
}

// CreateSigningKeyVersion creates a new signing key version on the key manager
// and saves a reference to the new key version in the database. If creating the
// key in the key manager fails, the database is not updated. However, if
// updating the signing key in the database fails, the key is NOT deleted from
// the key manager.
func (r *Realm) CreateSigningKeyVersion(ctx context.Context, db *Database) (string, error) {
	manager := db.signingKeyManager
	if manager == nil {
		return "", ErrNoSigningKeyManager
	}

	parent := db.config.CertificateSigningKeyRing
	if parent == "" {
		return "", fmt.Errorf("missing CERTIFICATE_SIGNING_KEYRING")
	}

	name := r.SigningKeyID()
	if name == "" {
		return "", fmt.Errorf("missing key name")
	}

	// Create the parent key - this interface does not return an error if the key
	// already exists, so this is safe to run each time.
	keyName, err := manager.CreateSigningKey(ctx, parent, name)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Create a new key version. This returns the full version name.
	version, err := manager.CreateKeyVersion(ctx, keyName)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key version: %w", err)
	}

	// Drop a log message for debugging.
	db.logger.Debugw("provisioned new signing key for realm",
		"realm_id", r.ID,
		"key_id", version)

	// Save the reference to the key in the database. This is done in a
	// transaction to avoid a race where keys are being created simultaneously and
	// both are set to active.
	var signingKey SigningKey
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Look and see if there are existing signing keys for this realm. We do
		// this to determine if the new key should be set to "active" automatically
		// or if the user needs to take manual action to move the pointer.
		var count int64
		if err := tx.
			Table("signing_keys").
			Where("realm_id = ?", r.ID).
			Count(&count).
			Error; err != nil {
			if !IsNotFound(err) {
				return fmt.Errorf("failed to check for existing keys: %w", err)
			}
		}

		// Create the new key.
		signingKey.RealmID = r.ID
		signingKey.KeyID = version
		signingKey.Active = (count == 0)

		// Save the key.
		if err := tx.Save(&signingKey).Error; err != nil {
			return fmt.Errorf("failed to save reference to signing key: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}

	return signingKey.GetKID(), nil
}

// DestroySigningKeyVersion destroys the given key version in both the database
// and the key manager. ID is the primary key ID from the database. If the id
// does not exist, it does nothing.
func (r *Realm) DestroySigningKeyVersion(ctx context.Context, db *Database, id interface{}) error {
	manager := db.signingKeyManager
	if manager == nil {
		return ErrNoSigningKeyManager
	}

	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Load the signing key to ensure it actually exists.
		var signingKey SigningKey
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Table("signing_keys").
			Where("id = ?", id).
			Where("realm_id = ?", r.ID).
			First(&signingKey).
			Error; err != nil {
			if IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to load signing key: %w", err)
		}

		if signingKey.Active {
			return fmt.Errorf("cannot destroy active signing key")
		}

		// Delete the signing key from the key manager - we want to do this in the
		// transaction so, if it fails, we can rollback and try again.
		if err := manager.DestroyKeyVersion(ctx, signingKey.KeyID); err != nil {
			return fmt.Errorf("failed to destroy signing key in key manager: %w", err)
		}

		// Successfully deleted from the key manager, now remove the record.
		if err := tx.Delete(&signingKey).Error; err != nil {
			return fmt.Errorf("successfully destroyed signing key in key manager, "+
				"but failed to delete signing key from database: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to destroy signing key version: %w", err)
	}

	return nil
}
