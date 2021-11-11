// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
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
	APIKeyTypeStats
)

func (a APIKeyType) Display() string {
	switch a {
	case APIKeyTypeInvalid:
		return "invalid"
	case APIKeyTypeDevice:
		return "device"
	case APIKeyTypeAdmin:
		return "admin"
	case APIKeyTypeStats:
		return "stats"
	default:
		return "invalid"
	}
}

var _ Auditable = (*AuthorizedApp)(nil)

// AuthorizedApp represents an application that is authorized to verify
// verification codes and perform token exchanges.
// This is controlled via a generated API key.
//
// Admin Keys are able to issue diagnosis keys and are not able to participate
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

	// LastUsedAt is the estimated time at which the API key was last used. For
	// performance reasons, this not incremented on each use but rather in short
	// buckets to avoid a write on every read.
	LastUsedAt *time.Time `gorm:"column:last_used_at; type:timestamp with time zone;"`
}

// BeforeSave runs validations. If there are errors, the save fails.
func (a *AuthorizedApp) BeforeSave(tx *gorm.DB) error {
	a.Name = project.TrimSpace(a.Name)

	if a.Name == "" {
		a.AddError("name", "cannot be blank")
	}

	if !(a.APIKeyType == APIKeyTypeDevice || a.APIKeyType == APIKeyTypeAdmin || a.APIKeyType == APIKeyTypeStats) {
		a.AddError("type", "is invalid")
	}

	return a.ErrorOrNil()
}

func (a *AuthorizedApp) IsAdminType() bool {
	return a.APIKeyType == APIKeyTypeAdmin
}

func (a *AuthorizedApp) IsDeviceType() bool {
	return a.APIKeyType == APIKeyTypeDevice
}

func (a *AuthorizedApp) IsStatsType() bool {
	return a.APIKeyType == APIKeyTypeStats
}

// Realm returns the associated realm for this app. If you only need the ID,
// call .RealmID instead of a full database lookup.
func (a *AuthorizedApp) Realm(db *Database) (*Realm, error) {
	var realm Realm
	if err := db.db.
		Model(&Realm{}).
		Where("id = ?", a.RealmID).
		First(&realm).
		Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

// FindAuthorizedApp finds the authorized app by the given id.
func (db *Database) FindAuthorizedApp(id interface{}) (*AuthorizedApp, error) {
	var app AuthorizedApp
	if err := db.db.
		Unscoped().
		Model(AuthorizedApp{}).
		Order("LOWER(name) ASC").
		Where("id = ?", id).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
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
func (a *AuthorizedApp) Stats(db *Database) (AuthorizedAppStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	// Pull the stats by generating the full date range, then join on stats. This
	// will ensure we have a full list (with values of 0 where appropriate) to
	// ensure continuity in graphs.
	sql := `
		SELECT
			d.date AS date,
			$1 AS authorized_app_id,
			$2 AS authorized_app_name,
			$3 AS authorized_app_type,
			COALESCE(s.codes_issued, 0) AS codes_issued,
			COALESCE(s.codes_claimed, 0) AS codes_claimed,
			COALESCE(s.codes_invalid, 0) AS codes_invalid,
			COALESCE(s.tokens_claimed, 0) AS tokens_claimed,
			COALESCE(s.tokens_invalid, 0) AS tokens_invalid
		FROM (
			SELECT date::date FROM generate_series($4, $5, '1 day'::interval) date
		) d
		LEFT JOIN authorized_app_stats s ON s.authorized_app_id = $1 AND s.date = d.date
		ORDER BY date DESC`

	var stats []*AuthorizedAppStat
	if err := db.db.Raw(sql, a.ID, a.Name, a.APIKeyType.Display(), start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// StatsCached is stats, but cached.
func (a *AuthorizedApp) StatsCached(ctx context.Context, db *Database, cacher cache.Cacher) (AuthorizedAppStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats AuthorizedAppStats
	cacheKey := &cache.Key{
		Namespace: "stats:app",
		Key:       strconv.FormatUint(uint64(a.ID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return a.Stats(db)
	}); err != nil {
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
		return ErrMissingActor
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
			return fmt.Errorf("failed to get existing API key: %w", err)
		}

		// Save the app
		if err := tx.Unscoped().Save(a).Error; err != nil {
			if IsUniqueViolation(err, "realm_apikey_name") {
				a.AddError("name", "must be unique")
				return ErrValidationFailed
			}
			return err
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
	keys, err := db.GetAPIKeyDatabaseHMAC()
	if err != nil {
		return "", fmt.Errorf("failed get keys to generate API key database HMAC: %w", err)
	}
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
	keys, err := db.GetAPIKeyDatabaseHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys to generate API key database HMACs: %w", err)
	}

	sigs := make([]string, 0, len(keys))
	for _, key := range keys {
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
	key = key + "." + sig

	return key, nil
}

// GenerateAPIKeySignature returns all possible signatures of the given key.
func (db *Database) GenerateAPIKeySignature(apiKey string) (string, error) {
	keys, err := db.GetAPIKeySignatureHMAC()
	if err != nil {
		return "", fmt.Errorf("failed to get keys to generate API key signature HMAC: %w", err)
	}

	return initialHMAC(keys, apiKey)
}

// generateAPIKeySignatures returns all possible signatures of the given key.
func (db *Database) generateAPIKeySignatures(apiKey string) ([][]byte, error) {
	keys, err := db.GetAPIKeySignatureHMAC()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys to generate API key signature HMACs: %w", err)
	}

	allSigs, err := allAllowedHMACs(keys, apiKey)
	if err != nil {
		return nil, err
	}
	asBytes := make([][]byte, 0, len(allSigs))
	for _, b64sig := range allSigs {
		sig, err := base64util.DecodeString(b64sig)
		if err != nil {
			return nil, fmt.Errorf("error decoding API key signature: %w", err)
		}
		asBytes = append(asBytes, sig)
	}
	return asBytes, nil
}

// VerifyAPIKeySignature verifies the signature matches the expected value for
// the key. It does this by computing the expected signature and then doing a
// constant-time comparison against the provided signature.
func (db *Database) VerifyAPIKeySignature(key string) (string, uint64, error) {
	logger := db.logger.Named("VerifyAPIKeySignature")

	key = project.TrimSpaceAndNonPrintable(key)

	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid API key format: wrong number of parts")
	}

	// Decode the provided signature.
	gotSigStr := parts[2]
	gotSig, err := base64util.DecodeString(gotSigStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format: decoding failed: %w", err)
	}

	gotKey := parts[0]
	if gotKey == "" {
		return "", 0, fmt.Errorf("invalid API key format: missing API key")
	}

	gotRealm := parts[1]
	if gotRealm == "" {
		return "", 0, fmt.Errorf("invalid API key format: missing realm")
	}

	// Generate the expected signature.
	expSigs, err := db.generateAPIKeySignatures(gotKey + "." + gotRealm)
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format: signature generation: %w", err)
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
		logger.Debugw("API key signature did not match any expected values",
			"got", gotSig,
			"expected", expSigs)
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

// TouchLastUsedAt updates the timestamp at which the authorized app was last
// used. It does not write an audit entry.
func (a *AuthorizedApp) TouchLastUsedAt(db *Database) error {
	now := time.Now().UTC()
	a.LastUsedAt = &now
	if err := db.db.
		Model(&AuthorizedApp{}).
		Update(a).
		Error; err != nil {
		return fmt.Errorf("failed to update last_used_at: %w", err)
	}
	return nil
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
