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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/jinzhu/gorm"
)

const (
	apiKeyBytes = 64 // 64 bytes is 86 chararacters in non-padded base64.
)

type APIKeyType int

const (
	APIKeyTypeInvalid APIKeyType = iota - 1
	APIKeyTypeDevice
	APIKeyTypeAdmin
)

func (a APIKeyType) Display() string {
	switch a {
	case APIKeyTypeDevice:
		return "device"
	case APIKeyTypeAdmin:
		return "admin"
	default:
		return "invalid"
	}
}

var _ Auditable = (*AuthorizedApp)(nil)

// AuthorizedApp represents an application that is authorized to verify
// verification codes and perform token exchanges.
// This is controlled via a generated API key.
//
// Admin Keys are able to issue diagnosis keys and are not able to perticipate
// the verification protocol.
type AuthorizedApp struct {
	gorm.Model
	Errorable

	// AuthorizedApps belong to exactly one realm.
	RealmID uint `gorm:"unique_index:realm_apikey_name"`

	// Name is the name of the authorized app.
	Name string `gorm:"type:varchar(100);unique_index:realm_apikey_name"`

	// APIKeyPreview is the first few characters of the API key for display
	// purposes. This can help admins in the UI when determining which API key is
	// in use.
	APIKeyPreview string `gorm:"type:varchar(32)"`

	// APIKey is the HMACed API key.
	APIKey string `gorm:"type:varchar(512);unique_index"`

	// APIKeyType is the API key type.
	APIKeyType APIKeyType `gorm:"column:api_key_type; type:integer; not null;"`
}

// BeforeSave runs validations. If there are errors, the save fails.
func (a *AuthorizedApp) BeforeSave(tx *gorm.DB) error {
	a.Name = project.TrimSpace(a.Name)

	if a.Name == "" {
		a.AddError("name", "cannot be blank")
	}

	if !(a.APIKeyType == APIKeyTypeDevice || a.APIKeyType == APIKeyTypeAdmin) {
		a.AddError("type", "is invalid")
	}

	if len(a.Errors()) > 0 {
		return fmt.Errorf("validation failed")
	}
	return nil
}

func (a *AuthorizedApp) IsAdminType() bool {
	return a.APIKeyType == APIKeyTypeAdmin
}

func (a *AuthorizedApp) IsDeviceType() bool {
	return a.APIKeyType == APIKeyTypeDevice
}

// Realm returns the associated realm for this app.
func (a *AuthorizedApp) Realm(db *Database) (*Realm, error) {
	var realm Realm
	if err := db.db.Model(a).Related(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

// TableName definition for the authorized apps relation.
func (AuthorizedApp) TableName() string {
	return "authorized_apps"
}

// CreateAuthorizedApp generates a new API key and assigns it to the specified
// app. Note that the API key is NOT stored in the database, only a hash. The
// only time the API key is available is as the string return parameter from
// invoking this function.
func (r *Realm) CreateAuthorizedApp(db *Database, app *AuthorizedApp, actor Auditable) (string, error) {
	fullAPIKey, err := db.GenerateAPIKey(r.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	parts := strings.SplitN(fullAPIKey, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("internal error, key is invalid")
	}
	apiKey := parts[0]

	hmacedKey, err := db.GenerateAPIKeyHMAC(apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to create hmac: %w", err)
	}

	app.RealmID = r.ID
	app.APIKey = hmacedKey
	app.APIKeyPreview = apiKey[:6]

	if err := db.SaveAuthorizedApp(app, actor); err != nil {
		return "", err
	}
	return fullAPIKey, nil
}

// FindAuthorizedAppByAPIKey located an authorized app based on API key.
func (db *Database) FindAuthorizedAppByAPIKey(apiKey string) (*AuthorizedApp, error) {
	logger := db.logger.Named("FindAuthorizedAppByAPIKey")

	// Determine if this is a v1 or v2 key. v2 keys have colons (v1 do not).
	if strings.Contains(apiKey, ".") {
		// v2 API keys are HMACed in the database.
		apiKey, realmID, err := db.VerifyAPIKeySignature(apiKey)
		if err != nil {
			logger.Warnw("failed to verify api key signature", "error", err)
			return nil, gorm.ErrRecordNotFound
		}

		hmacedKeys, err := db.generateAPIKeyHMACs(apiKey)
		if err != nil {
			logger.Warnw("failed to create hmac", "error", err)
			return nil, gorm.ErrRecordNotFound
		}

		// Find the API key that matches the constraints.
		var app AuthorizedApp
		if err := db.db.
			Where("api_key IN (?)", hmacedKeys).
			Where("realm_id = ?", realmID).
			First(&app).
			Error; err != nil {
			return nil, err
		}
		return &app, nil
	}

	// The API key is either invalid or a v1 API key.
	hmacedKeys, err := db.generateAPIKeyHMACs(apiKey)
	if err != nil {
		logger.Warnw("failed to create hmac", "error", err)
		return nil, gorm.ErrRecordNotFound
	}

	var app AuthorizedApp
	if err := db.db.
		Or("api_key IN (?)", hmacedKeys).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// Stats returns the usage statistics for this app. If no stats exist, it
// returns an empty array.
func (a *AuthorizedApp) Stats(db *Database, start, stop time.Time) ([]*AuthorizedAppStats, error) {
	var stats []*AuthorizedAppStats

	start = timeutils.UTCMidnight(start)
	stop = timeutils.UTCMidnight(stop)

	if err := db.db.
		Model(&AuthorizedAppStats{}).
		Where("authorized_app_id = ?", a.ID).
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

// SaveAuthorizedApp saves the authorized app.
func (db *Database) SaveAuthorizedApp(a *AuthorizedApp, actor Auditable) error {
	if a == nil {
		return fmt.Errorf("provided API key is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing AuthorizedApp
		if err := tx.
			Unscoped().
			Model(&AuthorizedApp{}).
			Where("id = ?", a.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing API key")
		}

		// Save the app
		if err := tx.Unscoped().Save(a).Error; err != nil {
			return fmt.Errorf("failed to save API key: %w", err)
		}

		// Brand new app?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created API key", a, a.RealmID)
			audits = append(audits, audit)
		} else {
			if existing.Name != a.Name {
				audit := BuildAuditEntry(actor, "updated API key name", a, a.RealmID)
				audit.Diff = stringDiff(existing.Name, a.Name)
				audits = append(audits, audit)
			}

			if existing.DeletedAt != a.DeletedAt {
				audit := BuildAuditEntry(actor, "updated API key enabled", a, a.RealmID)
				audit.Diff = boolDiff(existing.DeletedAt == nil, a.DeletedAt == nil)
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

// GenerateAPIKeyHMAC generates the HMAC of the provided API key using the
// latest HMAC key.
func (db *Database) GenerateAPIKeyHMAC(apiKey string) (string, error) {
	keys := db.config.APIKeyDatabaseHMAC
	if len(keys) < 1 {
		return "", fmt.Errorf("expected at least 1 hmac key")
	}

	sig := hmac.New(sha512.New, keys[0])
	if _, err := sig.Write([]byte(apiKey)); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sig.Sum(nil)), nil
}

// generateAPIKeyHMACs creates a permutation of all possible API keys based on
// the provided HMACs. It's primarily used to find an API key in the database.
func (db *Database) generateAPIKeyHMACs(apiKey string) ([]string, error) {
	sigs := make([]string, 0, len(db.config.APIKeyDatabaseHMAC))
	for _, key := range db.config.APIKeyDatabaseHMAC {
		sig := hmac.New(sha512.New, key)
		if _, err := sig.Write([]byte(apiKey)); err != nil {
			return nil, err
		}
		sigs = append(sigs, base64.RawURLEncoding.EncodeToString(sig.Sum(nil)))
	}
	return sigs, nil
}

// GenerateAPIKey generates a new API key that is bound to the given realm. This
// API key is NOT stored in the database. API keys are of the format:
//
//   key:realmID:hex(hmac)
//
func (db *Database) GenerateAPIKey(realmID uint) (string, error) {
	// Create the "key" parts.
	buf := make([]byte, apiKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to rand: %w", err)
	}
	key := base64.RawURLEncoding.EncodeToString(buf)

	// Add the realm ID.
	key = key + "." + strconv.FormatUint(uint64(realmID), 10)

	// Create the HMAC of the key and the realm.
	sig, err := db.GenerateAPIKeySignature(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	// Put it all together.
	key = key + "." + base64.RawURLEncoding.EncodeToString(sig)

	return key, nil
}

// GenerateAPIKeySignature returns all possible signatures of the given key.
func (db *Database) GenerateAPIKeySignature(apiKey string) ([]byte, error) {
	keys := db.config.APIKeySignatureHMAC
	if len(keys) < 1 {
		return nil, fmt.Errorf("expected at least 1 hmac key")
	}
	sig := hmac.New(sha512.New, keys[0])
	if _, err := sig.Write([]byte(apiKey)); err != nil {
		return nil, err
	}
	return sig.Sum(nil), nil
}

// generateAPIKeySignatures returns all possible signatures of the given key.
func (db *Database) generateAPIKeySignatures(apiKey string) ([][]byte, error) {
	sigs := make([][]byte, 0, len(db.config.APIKeySignatureHMAC))
	for _, key := range db.config.APIKeySignatureHMAC {
		sig := hmac.New(sha512.New, key)
		if _, err := sig.Write([]byte(apiKey)); err != nil {
			return nil, err
		}
		sigs = append(sigs, sig.Sum(nil))
	}
	return sigs, nil
}

// VerifyAPIKeySignature verifies the signature matches the expected value for
// the key. It does this by computing the expected signature and then doing a
// constant-time comparison against the provided signature.
func (db *Database) VerifyAPIKeySignature(key string) (string, uint64, error) {
	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid API key format: wrong number of parts")
	}

	// Decode the provided signature.
	gotSig, err := base64util.DecodeString(parts[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format: decoding failed")
	}

	// Generate the expected signature.
	expSigs, err := db.generateAPIKeySignatures(parts[0] + "." + parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format: signature invalid")
	}

	found := false
	for _, expSig := range expSigs {
		// Compare (this is an equal-time algorithm).
		if hmac.Equal(gotSig, expSig) {
			found = true
			// break // No! Don't break - we want constant time!
		}
	}

	if !found {
		return "", 0, fmt.Errorf("invalid API key format: signature invalid")
	}

	// API key stays encoded.
	apiKey := parts[0]

	// If we got this far, validation succeeded, parse the realm as a uint.
	realmID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format")
	}

	return apiKey, realmID, nil
}

func (a *AuthorizedApp) AuditID() string {
	return fmt.Sprintf("authorized_apps:%d", a.ID)
}

func (a *AuthorizedApp) AuditDisplay() string {
	return a.Name
}

// PurgeAuthorizedApps will delete authorized apps that have been deleted for
// more than the specified time.
func (db *Database) PurgeAuthorizedApps(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", deleteBefore).
		Delete(&AuthorizedApp{})
	return result.RowsAffected, result.Error
}
