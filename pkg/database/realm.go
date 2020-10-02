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
	"math"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/microcosm-cc/bluemonday"

	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
	"github.com/russross/blackfriday/v2"
)

// TestType is a test type in the database.
type TestType int16

const (
	_ TestType = 1 << iota
	TestTypeConfirmed
	TestTypeLikely
	TestTypeNegative
)

func (t TestType) Display() string {
	var types []string

	if t&TestTypeConfirmed != 0 {
		types = append(types, "confirmed")
	}

	if t&TestTypeLikely != 0 {
		types = append(types, "likely")
	}

	if t&TestTypeNegative != 0 {
		types = append(types, "negative")
	}

	return strings.Join(types, ", ")
}

var (
	ErrNoSigningKeyManagement = errors.New("no signing key management")
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

// AuthRequirement represents authentication requirements for the realm
type AuthRequirement int16

const (
	// MFAOptionalPrompt will prompt users for MFA on login.
	MFAOptionalPrompt = iota
	// MFARequired will not allow users to proceed without MFA on their account.
	MFARequired
	// MFAOptional will not prompt users to enable MFA.
	MFAOptional

	// MaxPageSize is the maximum allowed page size for a list query.
	MaxPageSize = 1000
)

var _ Auditable = (*Realm)(nil)

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	gorm.Model
	Errorable

	// Name is the name of the realm.
	Name string `gorm:"type:varchar(200);unique_index"`

	// RegionCode is both a display attribute and required field for ENX. To
	// handle NULL and uniqueness, the field is converted from it's ptr type to a
	// concrete type in callbacks. Do not modify RegionCodePtr directly.
	RegionCode    string  `gorm:"-"`
	RegionCodePtr *string `gorm:"column:region_code; type:varchar(10);"`

	// WelcomeMessage is arbitrary realm-defined data to display to users after
	// selecting this realm. If empty, nothing is displayed. The format is
	// markdown. Do not modify WelcomeMessagePtr directly.
	WelcomeMessage    string  `gorm:"-"`
	WelcomeMessagePtr *string `gorm:"column:welcome_message; type:text;"`

	// Code configuration
	CodeLength       uint            `gorm:"type:smallint; not null; default: 8"`
	CodeDuration     DurationSeconds `gorm:"type:bigint; not null; default: 900"` // default 15m (in seconds)
	LongCodeLength   uint            `gorm:"type:smallint; not null; default: 16"`
	LongCodeDuration DurationSeconds `gorm:"type:bigint; not null; default: 86400"` // default 24h

	// SMS configuration
	SMSTextTemplate string `gorm:"type:varchar(400); not null; default: 'This is your Exposure Notifications Verification code: [longcode] Expires in [longexpires] hours'"`

	// SMSCountry is an optional field to hint the default phone picker country
	// code.
	SMSCountry    string  `gorm:"-"`
	SMSCountryPtr *string `gorm:"column:sms_country; type:varchar(5);"`

	// CanUseSystemSMSConfig is configured by system administrators to share the
	// system SMS config with this realm. Note that the system SMS config could be
	// empty and a local SMS config is preferred over the system value.
	CanUseSystemSMSConfig bool `gorm:"column:can_use_system_sms_config; type:bool; not null; default:false;"`

	// UseSystemSMSConfig is a realm-level configuration that lets a realm opt-out
	// of sending SMS messages using the system-provided SMS configuration.
	// Without this, a realm would always fallback to the system-level SMS
	// configuration, making it impossible to opt out of text message sending.
	UseSystemSMSConfig bool `gorm:"column:use_system_sms_config; type:bool; not null; default:false;"`

	// MFAMode represents the mode for Multi-Factor-Authorization requirements for the realm.
	MFAMode AuthRequirement `gorm:"type:smallint; not null; default: 0"`

	// MFARequiredGracePeriod defines how long after creation a user may skip adding
	// a second auth factor before the server requires it.
	MFARequiredGracePeriod DurationSeconds `gorm:"type:bigint; not null; default: 0"`

	// EmailVerifiedMode represents the mode for email verification requirements for the realm.
	EmailVerifiedMode AuthRequirement `gorm:"type:smallint; not null; default: 0"`

	// PasswordRotationPeriodDays is the number of days before the user must
	// rotate their password.
	PasswordRotationPeriodDays uint `gorm:"type:smallint; not null; default: 0"`

	// PasswordRotationWarningDays is the number of days before Password expiry
	// that the user should receive a warning.
	PasswordRotationWarningDays uint `gorm:"type:smallint; not null; default: 0"`

	// AllowedCIDRs is the list of allowed IPs to the various services.
	AllowedCIDRsAdminAPI  pq.StringArray `gorm:"column:allowed_cidrs_adminapi; type:varchar(50)[];"`
	AllowedCIDRsAPIServer pq.StringArray `gorm:"column:allowed_cidrs_apiserver; type:varchar(50)[];"`
	AllowedCIDRsServer    pq.StringArray `gorm:"column:allowed_cidrs_server; type:varchar(50)[];"`

	// AllowedTestTypes is the type of tests that this realm permits. The default
	// value is to allow all test types.
	AllowedTestTypes TestType `gorm:"type:smallint; not null; default: 14"`

	// RequireDate requires that verifications on this realm require a test or
	// symptom date (either). The default behavior is to not require a date.
	RequireDate bool `gorm:"type:boolean; not null; default:false"`

	// Signing Key Settings
	UseRealmCertificateKey bool            `gorm:"type:boolean; default: false"`
	CertificateIssuer      string          `gorm:"type:varchar(150); default: ''"`
	CertificateAudience    string          `gorm:"type:varchar(150); default: ''"`
	CertificateDuration    DurationSeconds `gorm:"type:bigint; default: 900"` // 15m

	// EN Express
	EnableENExpress bool `gorm:"type:boolean; default: false"`

	// AbusePreventionEnabled determines if abuse protection is enabled.
	AbusePreventionEnabled bool `gorm:"type:boolean; not null; default:false"`

	// AbusePreventionLimit is the configured daily limit for the realm. This value is populated
	// by the nightly aggregation job and is based on a statistical model from
	// historical code issuance data.
	AbusePreventionLimit uint `gorm:"type:integer; not null; default:10"`

	// AbusePreventionLimitFactor is the factor against the predicted model for the day which
	// determines the total number of codes that can be issued for the realm on
	// the day. For example, if the predicted value was 50 and this value was 1.5,
	// the realm could generate 75 codes today before triggering abuse prevention.
	// Similarly, if this value was 0.5, the realm could only generate 25 codes
	// before triggering abuse protections.
	AbusePreventionLimitFactor float32 `gorm:"type:numeric(6, 3); not null; default:1.0"`

	// These are here for gorm to setup the association. You should NOT call them
	// directly, ever. Use the ListUsers function instead. The have to be public
	// for reflection.
	RealmUsers  []*User `gorm:"many2many:user_realms; PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
	RealmAdmins []*User `gorm:"many2many:admin_realms; PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`

	// Relations to items that belong to a realm.
	Codes  []*VerificationCode `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
	Tokens []*Token            `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false"`
}

// EffectiveMFAMode returns the realm's default MFAMode but first
// checks if the user is in the grace-period (if so, required becomes promp).
func (r *Realm) EffectiveMFAMode(user *User) AuthRequirement {
	if r == nil {
		return MFARequired
	}

	if time.Since(user.CreatedAt) <= r.MFARequiredGracePeriod.Duration {
		return MFAOptionalPrompt
	}
	return r.MFAMode
}

func (mode *AuthRequirement) String() string {
	switch *mode {
	case MFAOptionalPrompt:
		return "prompt"
	case MFARequired:
		return "required"
	case MFAOptional:
		return "optional"
	}
	return ""
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
		SMSTextTemplate:     "This is your Exposure Notifications Verification code: [longcode] Expires in [longexpires] hours",
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

// AfterFind runs after a realm is found.
func (r *Realm) AfterFind(tx *gorm.DB) error {
	r.RegionCode = stringValue(r.RegionCodePtr)
	r.WelcomeMessage = stringValue(r.WelcomeMessagePtr)
	r.SMSCountry = stringValue(r.SMSCountryPtr)

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
	r.RegionCodePtr = stringPtr(r.RegionCode)

	r.WelcomeMessage = strings.TrimSpace(r.WelcomeMessage)
	r.WelcomeMessagePtr = stringPtr(r.WelcomeMessage)

	if r.UseSystemSMSConfig && !r.CanUseSystemSMSConfig {
		r.AddError("useSystemSMSConfig", "is not allowed on this realm")
	}

	r.SMSCountryPtr = stringPtr(r.SMSCountry)

	if r.EnableENExpress {
		if r.RegionCode == "" {
			r.AddError("regionCode", "cannot be blank when using EN Express")
		}
	}

	if r.PasswordRotationWarningDays > r.PasswordRotationPeriodDays {
		r.AddError("passwordWarn", "may not be longer than password rotation period")
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
		return fmt.Errorf("realm validation failed: %s", strings.Join(r.ErrorMessages(), ", "))
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
func (r *Realm) BuildSMSText(code, longCode string, enxDomain string) string {
	text := r.SMSTextTemplate

	if enxDomain == "" {
		// preserves legacy behavior.
		text = strings.ReplaceAll(text, SMSENExpressLink, fmt.Sprintf("ens://v?r=%s&c=%s", SMSRegion, SMSLongCode))
	} else {
		text = strings.ReplaceAll(text, SMSENExpressLink,
			fmt.Sprintf("https://%s.%s/v?c=%s",
				strings.ToLower(r.RegionCode),
				enxDomain,
				SMSLongCode))
	}
	text = strings.ReplaceAll(text, SMSRegion, r.RegionCode)
	text = strings.ReplaceAll(text, SMSCode, code)
	text = strings.ReplaceAll(text, SMSExpires, fmt.Sprintf("%d", r.GetCodeDurationMinutes()))
	text = strings.ReplaceAll(text, SMSLongCode, longCode)
	text = strings.ReplaceAll(text, SMSLongExpires, fmt.Sprintf("%d", r.GetLongCodeDurationHours()))

	return text
}

// SMSConfig returns the SMS configuration for this realm, if one exists. If the
// realm is configured to use the system SMS configuration, that configuration
// is preferred.
func (r *Realm) SMSConfig(db *Database) (*SMSConfig, error) {
	q := db.db.
		Model(&SMSConfig{}).
		Order("is_system DESC").
		Where("realm_id = ?", r.ID)

	if r.UseSystemSMSConfig {
		q = q.Or("is_system IS TRUE")
	}

	var smsConfig SMSConfig
	if err := q.First(&smsConfig).Error; err != nil {
		return nil, err
	}
	return &smsConfig, nil
}

// HasSMSConfig returns true if the realm has an SMS config, false otherwise.
// This does not perform the KMS encryption/decryption, so it's more efficient
// that loading the full SMS config.
func (r *Realm) HasSMSConfig(db *Database) (bool, error) {
	q := db.db.
		Model(&SMSConfig{}).
		Select("id").
		Order("is_system DESC").
		Where("realm_id = ?", r.ID)

	if r.UseSystemSMSConfig {
		q = q.Or("is_system IS TRUE")
	}

	var id []uint64
	if err := q.Pluck("id", &id).Error; err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return len(id) > 0, nil
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

func (r *Realm) Audits(db *Database) ([]*AuditEntry, error) {
	var entries []*AuditEntry
	if err := db.db.
		Model(&AuditEntry{}).
		Where("realm_id = ?", r.ID).
		Order("created_at DESC").
		Find(&entries).
		Error; err != nil {
		if IsNotFound(err) {
			return entries, nil
		}
		return nil, err
	}
	return entries, nil
}

// AbusePreventionEffectiveLimit returns the effective limit, multiplying the limit by the
// limit factor and rounding up.
func (r *Realm) AbusePreventionEffectiveLimit() uint {
	// Only maintain 3 digits of precision, since that's all we do in the
	// database.
	factor := math.Floor(float64(r.AbusePreventionLimitFactor)*100) / 100
	return uint(math.Ceil(float64(r.AbusePreventionLimit) * float64(factor)))
}

// AbusePreventionEnabledRealmIDs returns the list of realm IDs that have abuse
// prevention enabled.
func (db *Database) AbusePreventionEnabledRealmIDs() ([]uint64, error) {
	var ids []uint64
	if err := db.db.
		Model(&Realm{}).
		Where("abuse_prevention_enabled IS true").
		Pluck("id", &ids).
		Error; err != nil {
		return nil, err
	}
	return ids, nil
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
		Order("LOWER(name) ASC").
		Where("id = ? AND realm_id = ?", id, r.ID).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListMobileApps gets all the mobile apps for the realm.
func (r *Realm) ListMobileApps(db *Database) ([]*MobileApp, error) {
	var apps []*MobileApp
	if err := db.db.
		Unscoped().
		Model(r).
		Order("mobile_apps.deleted_at DESC, LOWER(mobile_apps.name)").
		Related(&apps).
		Error; err != nil {
		if IsNotFound(err) {
			return apps, nil
		}
		return nil, err
	}
	return apps, nil
}

// FindMobileApp finds the mobile app by the given id associated with the realm.
func (r *Realm) FindMobileApp(db *Database, id interface{}) (*MobileApp, error) {
	var app MobileApp
	if err := db.db.
		Unscoped().
		Model(MobileApp{}).
		Where("id = ?", id).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// CountUsers returns the count users on this realm.
func (r *Realm) CountUsers(db *Database) (int, error) {
	var count int
	if err := db.db.
		Model(&User{}).
		Joins("INNER JOIN user_realms ON user_realms.user_id = users.id and realm_id = ?", r.ID).
		Count(&count).
		Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListUsers returns the list of users on this realm.
func (r *Realm) ListUsers(db *Database, offset, limit int, emailPrefix string) ([]*User, error) {
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	realmDB := db.db.Model(r)

	if emailPrefix != "" {
		realmDB = realmDB.Where("email like ?", fmt.Sprintf("%%%s%%", emailPrefix))
	}

	var users []*User
	if err := realmDB.
		Offset(offset).Limit(limit).
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
	if err := db.db.
		Order("name ASC").
		Find(&realms).
		Error; err != nil {
		return nil, err
	}
	return realms, nil
}

func (r *Realm) AuditID() string {
	return fmt.Sprintf("realms:%d", r.ID)
}

func (r *Realm) AuditDisplay() string {
	return r.Name
}

func (db *Database) SaveRealm(r *Realm, actor Auditable) error {
	if r == nil {
		return fmt.Errorf("provided realm is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing Realm
		if err := tx.
			Model(&Realm{}).
			Where("id = ?", r.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing realm")
		}

		// Save the realm
		if err := tx.Save(r).Error; err != nil {
			return fmt.Errorf("failed to save realm: %w", err)
		}

		// Brand new realm?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created realm", r, r.ID)
			audits = append(audits, audit)
		} else {
			if existing.Name != r.Name {
				audit := BuildAuditEntry(actor, "updated realm name", r, r.ID)
				audit.Diff = stringDiff(existing.Name, r.Name)
				audits = append(audits, audit)
			}

			if existing.RegionCode != r.RegionCode {
				audit := BuildAuditEntry(actor, "updated region code", r, r.ID)
				audit.Diff = stringDiff(existing.RegionCode, r.RegionCode)
				audits = append(audits, audit)
			}

			if existing.WelcomeMessage != r.WelcomeMessage {
				audit := BuildAuditEntry(actor, "updated welcome message", r, r.ID)
				audit.Diff = stringDiff(existing.WelcomeMessage, r.WelcomeMessage)
				audits = append(audits, audit)
			}

			if existing.CodeLength != r.CodeLength {
				audit := BuildAuditEntry(actor, "updated code length", r, r.ID)
				audit.Diff = uintDiff(existing.CodeLength, r.CodeLength)
				audits = append(audits, audit)
			}

			if existing.CodeDuration != r.CodeDuration {
				audit := BuildAuditEntry(actor, "updated code duration", r, r.ID)
				audit.Diff = stringDiff(existing.CodeDuration.AsString, r.CodeDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.LongCodeLength != r.LongCodeLength {
				audit := BuildAuditEntry(actor, "updated long code length", r, r.ID)
				audit.Diff = uintDiff(existing.LongCodeLength, r.LongCodeLength)
				audits = append(audits, audit)
			}

			if existing.LongCodeDuration != r.LongCodeDuration {
				audit := BuildAuditEntry(actor, "updated long code duration", r, r.ID)
				audit.Diff = stringDiff(existing.LongCodeDuration.AsString, r.LongCodeDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.SMSTextTemplate != r.SMSTextTemplate {
				audit := BuildAuditEntry(actor, "updated SMS template", r, r.ID)
				audit.Diff = stringDiff(existing.SMSTextTemplate, r.SMSTextTemplate)
				audits = append(audits, audit)
			}

			if existing.SMSCountry != r.SMSCountry {
				audit := BuildAuditEntry(actor, "updated SMS country", r, r.ID)
				audit.Diff = stringDiff(existing.SMSCountry, r.SMSCountry)
				audits = append(audits, audit)
			}

			if existing.CanUseSystemSMSConfig != r.CanUseSystemSMSConfig {
				audit := BuildAuditEntry(actor, "updated ability to use system SMS config", r, r.ID)
				audit.Diff = boolDiff(existing.CanUseSystemSMSConfig, r.CanUseSystemSMSConfig)
				audits = append(audits, audit)
			}

			if existing.UseSystemSMSConfig != r.UseSystemSMSConfig {
				audit := BuildAuditEntry(actor, "updated use system SMS config", r, r.ID)
				audit.Diff = boolDiff(existing.UseSystemSMSConfig, r.UseSystemSMSConfig)
				audits = append(audits, audit)
			}

			if existing.MFAMode != r.MFAMode {
				audit := BuildAuditEntry(actor, "updated MFA mode", r, r.ID)
				audit.Diff = stringDiff(existing.MFAMode.String(), r.MFAMode.String())
				audits = append(audits, audit)
			}

			if existing.MFARequiredGracePeriod != r.MFARequiredGracePeriod {
				audit := BuildAuditEntry(actor, "updated MFA required grace period", r, r.ID)
				audit.Diff = stringDiff(existing.MFARequiredGracePeriod.AsString, r.MFARequiredGracePeriod.AsString)
				audits = append(audits, audit)
			}

			if existing.EmailVerifiedMode != r.EmailVerifiedMode {
				audit := BuildAuditEntry(actor, "updated email verification mode", r, r.ID)
				audit.Diff = stringDiff(existing.EmailVerifiedMode.String(), r.EmailVerifiedMode.String())
				audits = append(audits, audit)
			}

			if existing.PasswordRotationPeriodDays != r.PasswordRotationPeriodDays {
				audit := BuildAuditEntry(actor, "updated password rotation period", r, r.ID)
				audit.Diff = uintDiff(existing.PasswordRotationPeriodDays, r.PasswordRotationPeriodDays)
				audits = append(audits, audit)
			}

			if existing.PasswordRotationWarningDays != r.PasswordRotationWarningDays {
				audit := BuildAuditEntry(actor, "updated password rotation warning", r, r.ID)
				audit.Diff = uintDiff(existing.PasswordRotationWarningDays, r.PasswordRotationWarningDays)
				audits = append(audits, audit)
			}

			// TODO(sethvargo): diff allowed CIDRs

			if existing.AllowedTestTypes != r.AllowedTestTypes {
				audit := BuildAuditEntry(actor, "updated allowed test types", r, r.ID)
				audit.Diff = stringDiff(existing.AllowedTestTypes.Display(), r.AllowedTestTypes.Display())
				audits = append(audits, audit)
			}

			if existing.RequireDate != r.RequireDate {
				audit := BuildAuditEntry(actor, "updated require date", r, r.ID)
				audit.Diff = boolDiff(existing.RequireDate, r.RequireDate)
				audits = append(audits, audit)
			}

			if existing.UseRealmCertificateKey != r.UseRealmCertificateKey {
				audit := BuildAuditEntry(actor, "updated use realm certificate key", r, r.ID)
				audit.Diff = boolDiff(existing.UseRealmCertificateKey, r.UseRealmCertificateKey)
				audits = append(audits, audit)
			}

			if existing.CertificateIssuer != r.CertificateIssuer {
				audit := BuildAuditEntry(actor, "updated certificate issuer", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateIssuer, r.CertificateIssuer)
				audits = append(audits, audit)
			}

			if existing.CertificateAudience != r.CertificateAudience {
				audit := BuildAuditEntry(actor, "updated certificate audience", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateAudience, r.CertificateAudience)
				audits = append(audits, audit)
			}

			if existing.CertificateDuration != r.CertificateDuration {
				audit := BuildAuditEntry(actor, "updated certificate duration", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateDuration.AsString, r.CertificateDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.EnableENExpress != r.EnableENExpress {
				audit := BuildAuditEntry(actor, "updated enable ENX", r, r.ID)
				audit.Diff = boolDiff(existing.EnableENExpress, r.EnableENExpress)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionEnabled != r.AbusePreventionEnabled {
				audit := BuildAuditEntry(actor, "updated enable abuse prevention", r, r.ID)
				audit.Diff = boolDiff(existing.AbusePreventionEnabled, r.AbusePreventionEnabled)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionLimit != r.AbusePreventionLimit {
				audit := BuildAuditEntry(actor, "updated abuse prevention limit", r, r.ID)
				audit.Diff = uintDiff(existing.AbusePreventionLimit, r.AbusePreventionLimit)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionLimitFactor != r.AbusePreventionLimitFactor {
				audit := BuildAuditEntry(actor, "updated abuse prevention limit factor", r, r.ID)
				audit.Diff = float32Diff(existing.AbusePreventionLimitFactor, r.AbusePreventionLimitFactor)
				audits = append(audits, audit)
			}
		}

		// Save all audits
		for _, audit := range audits {
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audits: %w", err)
			}
		}

		return nil
	})
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

// Stats returns the usage statistics for this realm. If no stats exist,
// returns an empty array.
func (r *Realm) Stats(db *Database, start, stop time.Time) ([]*RealmStats, error) {
	var stats []*RealmStats

	start = start.Truncate(24 * time.Hour)
	stop = stop.Truncate(24 * time.Hour)

	if err := db.db.
		Model(&RealmStats{}).
		Where("realm_id = ?", r.ID).
		Where("(date >= ? AND date <= ?)", start, stop).
		Order("date ASC").
		Find(&stats).
		Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}

	return stats, nil
}

// RenderWelcomeMessage message renders the realm's welcome message.
func (r *Realm) RenderWelcomeMessage() string {
	msg := strings.TrimSpace(r.WelcomeMessage)
	if msg == "" {
		return ""
	}

	raw := blackfriday.Run([]byte(msg))
	return string(bluemonday.UGCPolicy().SanitizeBytes(raw))
}

// ToCIDRList converts the newline-separated and/or comma-separated CIDR list
// into an array of strings.
func ToCIDRList(s string) ([]string, error) {
	var cidrs []string
	for _, line := range strings.Split(s, "\n") {
		for _, v := range strings.Split(line, ",") {
			v = strings.TrimSpace(v)

			// Ignore blanks
			if v == "" {
				continue
			}

			// If there's no /, assume the most specific. This is intentionally
			// rudimentary.
			if !strings.Contains(v, "/") {
				if strings.Contains(v, ":") {
					v = fmt.Sprintf("%s/128", v)
				} else {
					v = fmt.Sprintf("%s/32", v)
				}
			}

			// Basic sanity checking.
			if _, _, err := net.ParseCIDR(v); err != nil {
				return nil, err
			}

			cidrs = append(cidrs, v)
		}
	}

	sort.Strings(cidrs)
	return cidrs, nil
}
