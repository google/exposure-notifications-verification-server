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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/jinzhu/gorm"
)

type APIUserType int

const (
	apiKeyBytes = 64 // 64 bytes is 86 chararacters in non-padded base64.

	APIUserTypeDevice APIUserType = 0
	APIUserTypeAdmin  APIUserType = 1
)

// AuthorizedApp represents an application that is authorized to verify
// verification codes and perform token exchanges.
// This is controlled via a generated API key.
//
// Admin Keys are able to issue diagnosis keys and are not able to perticipate
// the verification protocol.
type AuthorizedApp struct {
	gorm.Model
	// AuthorizedApps belong to exactly one realm.
	RealmID uint   `gorm:"unique_index:realm_apikey_name"`
	Realm   *Realm // for loading the owning realm.
	Name    string `gorm:"type:varchar(100);unique_index:realm_apikey_name"`

	// APIKeyPreview is the first few characters of the API key for display
	// purposes. This can help admins in the UI when determining which API key is
	// in use.
	APIKeyPreview string `gorm:"type:varchar(32)"`

	// APIKey is the HMACed API key.
	APIKey string `gorm:"type:varchar(512);unique_index"`

	// APIKeyType s the API key type.
	APIKeyType APIUserType `gorm:"default:0"`
}

func (a *AuthorizedApp) IsAdminType() bool {
	return a.APIKeyType == APIUserTypeAdmin
}

func (a *AuthorizedApp) IsDeviceType() bool {
	return a.APIKeyType == APIUserTypeDevice
}

// GetRealm does a lazy load read of the realm associated with this
// authorized app.
func (a *AuthorizedApp) GetRealm(db *Database) (*Realm, error) {
	if a.Realm != nil {
		return a.Realm, nil
	}
	var realm Realm
	if err := db.db.Model(a).Related(&realm).Error; err != nil {
		return nil, err
	}
	a.Realm = &realm
	return a.Realm, nil
}

// TODO(mikehelmick): Implement revoke API key functionality.

// TableName definition for the authorized apps relation.
func (AuthorizedApp) TableName() string {
	return "authorized_apps"
}

// ListAuthorizedApps retrieves all of the configured authorized apps.
// Done without pagination, as the expected number of authorized apps
// is low signal digits.
func (db *Database) ListAuthorizedApps(includeDeleted bool) ([]*AuthorizedApp, error) {
	var apps []*AuthorizedApp

	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Preload("Realm").Order("LOWER(name) ASC").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("query authorized apps: %w", err)
	}
	return apps, nil
}

// CreateAuthorizedApp generates a new API key and assigns it to the specified
// app. Note that the API key is NOT stored in the database, only a hash. The
// only time the API key is available is as the string return parameter from
// invoking this function.
func (db *Database) CreateAuthorizedApp(realmID uint, name string, apiUserType APIUserType) (string, *AuthorizedApp, error) {
	if !(apiUserType == APIUserTypeAdmin || apiUserType == APIUserTypeDevice) {
		return "", nil, fmt.Errorf("invalid API Key user type requested: %v", apiUserType)
	}

	fullAPIKey, err := db.GenerateAPIKey(realmID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	parts := strings.SplitN(fullAPIKey, ".", 3)
	if len(parts) != 3 {
		return "", nil, fmt.Errorf("internal error, key is invalid")
	}
	apiKey := parts[0]

	hmacedKey, err := db.hmacAPIKey(apiKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var app AuthorizedApp
	app.RealmID = realmID
	app.Name = name
	app.APIKey = hmacedKey
	app.APIKeyPreview = apiKey[:6]
	app.APIKeyType = apiUserType

	if err := db.db.Create(&app).Error; err != nil {
		return "", nil, fmt.Errorf("failed to create api key: %w", err)
	}
	return fullAPIKey, &app, nil
}

// FindAuthorizedAppByAPIKey located an authorized app based on API key. If no
// app exists for the given API key, it returns nil.
func (db *Database) FindAuthorizedAppByAPIKey(apiKey string) (*AuthorizedApp, error) {
	// Determine if this is a v1 or v2 key. v2 keys have colons (v1 do not).
	if strings.Contains(apiKey, ".") {
		// v2 API keys are HMACed in the database.
		apiKey, realmID, err := db.VerifyAPIKeySignature(apiKey)
		if err != nil {
			return nil, err
		}

		hmacedKey, err := db.hmacAPIKey(apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create hmac: %w", err)
		}

		// Find the API key that matches the constraints.
		var app AuthorizedApp
		if err := db.db.
			Preload("Realm").
			Where("api_key = ?", hmacedKey).
			Where("realm_id = ?", realmID).
			First(&app).
			Error; err != nil {
			if gorm.IsRecordNotFoundError(err) || errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return &app, nil
	}

	// The API key is either invalid or a v1 API key. We need to check both the
	// HMACed value and the plaintext value since earlier versions of the API keys
	// were not HMACed.
	hmacedKey, err := db.hmacAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create hmac: %w", err)
	}

	var app AuthorizedApp
	if err := db.db.
		Preload("Realm").
		Where("api_key = ?", apiKey).
		// TODO(sethvargo): Remove the plaintext check after all keys have been hashed
		// in the database. We still need to keep the v1 path looking up by hmac
		// though, since v1 keys still exist.
		Or("api_key = ?", hmacedKey).
		First(&app).
		Error; err != nil {
		if gorm.IsRecordNotFoundError(err) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &app, nil
}

// SaveAuthorizedApp saves the authorized app.
func (db *Database) SaveAuthorizedApp(r *AuthorizedApp) error {
	if r.Model.ID == 0 {
		return db.db.Create(r).Error
	}
	return db.db.Save(r).Error
}

// hmacAPIKey is a helper for generating the HMAC of an API key. It returns the
// hex-encoded HMACed value, suitable for insertion into the database.
func (db *Database) hmacAPIKey(v string) (string, error) {
	sig := hmac.New(sha512.New, db.config.APIKeyDatabaseHMAC)
	if _, err := sig.Write([]byte(v)); err != nil {
		return "", nil
	}
	return base64.RawURLEncoding.EncodeToString(sig.Sum(nil)), nil
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

// GenerateAPIKeySignature signs the given API key using an HMAC shared secret.
func (db *Database) GenerateAPIKeySignature(key string) ([]byte, error) {
	sig := hmac.New(sha512.New, db.config.APIKeySignatureHMAC)
	if _, err := sig.Write([]byte(key)); err != nil {
		return nil, err
	}
	return sig.Sum(nil), nil
}

// VerifyAPIKeySignature verifies the signature matches the expected value for
// the key. It does this by computing the expected signature and then doing a
// constant-time comparison against the provided signature.
func (db *Database) VerifyAPIKeySignature(key string) (string, uint, error) {
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
	expSig, err := db.GenerateAPIKeySignature(parts[0] + "." + parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format: signature invalid")
	}

	// Compare (this is an equal-time algorithm).
	if !hmac.Equal(gotSig, expSig) {
		return "", 0, fmt.Errorf("invalid API key format: signature invalid")
	}

	// API key stays encoded.
	apiKey := parts[0]

	// If we got this far, validation succeeded, parse the realm as a uint.
	realmID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid API key format")
	}

	return apiKey, uint(realmID), nil
}
