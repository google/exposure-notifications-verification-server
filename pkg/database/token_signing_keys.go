// Copyright 2021 Google LLC
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
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
)

// TokenSigningKey represents a collection of references to a KMS-backed signing
// key version for verification token signing. It is also used to track rotation
// schedules.
type TokenSigningKey struct {
	Errorable

	// ID is the database auto-incrementing integer of the key.
	ID uint64

	// KeyVersionID is the full name of the signing key version.
	KeyVersionID string

	// UUID is the uuid of the key. This is used as the `kid` header value in
	// JWTs.
	UUID string `gorm:"column:uuid; default:null;"`

	// IsActive returns true if this signing key is the active one, false
	// otherwise. There's a database-level constraint that only one row can have
	// this value as true, so there is guaranteed to be exactly one active key at
	// a time.
	IsActive bool

	// CreatedAt is when the key was created and added to the system. UpdatedAt is
	// when the key was last updated, which includes marking it as inactive.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Ensure signing key can be an audited.
var _ Auditable = (*TokenSigningKey)(nil)

// AuditID is how the token signing key is stored in the audit entry.
func (k *TokenSigningKey) AuditID() string {
	return fmt.Sprintf("token_signing_key:%s", k.UUID)
}

// AuditDisplay is how the token signing key will be displayed in audit entries.
func (k *TokenSigningKey) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", k.UUID, k.KeyVersionID)
}

// FindTokenSigningKey finds the given key by database ID. It returns an error
// if the record is not found.
func (db *Database) FindTokenSigningKey(id interface{}) (*TokenSigningKey, error) {
	var key TokenSigningKey
	if err := db.db.
		Model(&TokenSigningKey{}).
		Where("id = ?", id).
		First(&key).
		Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// FindTokenSigningKeyByUUID finds the given key by database ID. It returns an error
// if the record is not found.
func (db *Database) FindTokenSigningKeyByUUID(uuidStr string) (*TokenSigningKey, error) {
	// Postgres returns an error if the provided input is not a valid UUID.
	parsed, err := uuid.Parse(uuidStr)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}

	var key TokenSigningKey
	if err := db.db.
		Model(&TokenSigningKey{}).
		Where("uuid = ?", parsed.String()).
		First(&key).
		Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// FindTokenSigningKeyByUUIDCached is like FindTokenSigningKeyByUUID, but the
// results are cached for a short period to alleviate load on the database.
func (db *Database) FindTokenSigningKeyByUUIDCached(ctx context.Context, cacher cache.Cacher, uuidStr string) (*TokenSigningKey, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var key *TokenSigningKey
	cacheKey := &cache.Key{
		Namespace: "token_signing_keys:by_id",
		Key:       uuidStr,
	}
	if err := cacher.Fetch(ctx, cacheKey, &key, 30*time.Minute, func() (interface{}, error) {
		return db.FindTokenSigningKeyByUUID(uuidStr)
	}); err != nil {
		return nil, err
	}
	return key, nil
}

// ActiveTokenSigningKey returns the currently-active token signing key. If no
// key is currently marked as active, it returns NotFound.
func (db *Database) ActiveTokenSigningKey() (*TokenSigningKey, error) {
	var key TokenSigningKey
	if err := db.db.
		Model(&TokenSigningKey{}).
		Where("is_active IS TRUE").
		First(&key).
		Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// ActiveTokenSigningKeyCached is like ActiveTokenSigningKey, but the results
// are cached for a short period to alleviate load on the database.
func (db *Database) ActiveTokenSigningKeyCached(ctx context.Context, cacher cache.Cacher) (*TokenSigningKey, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var key *TokenSigningKey
	cacheKey := &cache.Key{
		Namespace: "token_signing_keys:active",
		Key:       "value",
	}
	if err := cacher.Fetch(ctx, cacheKey, &key, 5*time.Minute, func() (interface{}, error) {
		return db.ActiveTokenSigningKey()
	}); err != nil {
		return nil, err
	}
	return key, nil
}

// ListTokenSigningKeys lists all keys sorted by their active state, then
// creation state descending. If there are no keys, it returns an empty list. To
// get the current active signing key, use ActiveTokenSigningKey.
func (db *Database) ListTokenSigningKeys() ([]*TokenSigningKey, error) {
	var keys []*TokenSigningKey
	if err := db.db.
		Model(&TokenSigningKey{}).
		Order("token_signing_keys.is_active DESC, id DESC").
		Find(&keys).
		Error; err != nil {
		if IsNotFound(err) {
			return keys, nil
		}
		return nil, err
	}
	return keys, nil
}

// SaveTokenSigningKey saves the token signing key.
func (db *Database) SaveTokenSigningKey(key *TokenSigningKey, actor Auditable) error {
	if key == nil {
		return fmt.Errorf("provided key is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		// Look up the existing record so we can do a diff and generate audit
		// entries.
		var existing TokenSigningKey
		if err := tx.
			Set("gorm::query_option", "FOR UPDATE").
			Model(&TokenSigningKey{}).
			Where("id = ?", key.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing token signing key")
		}

		// Save
		if err := tx.Save(key).Error; err != nil {
			return fmt.Errorf("failed to save token signing key: %w", err)
		}

		// Brand new?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created token signing key", key, 0)
			audits = append(audits, audit)
		} else {
			if then, now := existing.KeyVersionID, key.KeyVersionID; then != now {
				audit := BuildAuditEntry(actor, "updated key version id", key, 0)
				audit.Diff = stringDiff(then, now)
				audits = append(audits, audit)
			}

			if then, now := existing.IsActive, key.IsActive; then != now {
				audit := BuildAuditEntry(actor, "updated active state", key, 0)
				audit.Diff = boolDiff(then, now)
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

// ActivateTokenSigningKey activates the signing key with the provided database
// ID. If no record corresponds to the given ID, an error is returned. If the
// given ID is already active, no action is taken. Otherwise, all existing key
// versions are marked as inactive and this key is marked as active.
func (db *Database) ActivateTokenSigningKey(id interface{}, actor Auditable) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		// Lookup the existing key.
		var existing TokenSigningKey
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Model(&TokenSigningKey{}).
			Where("id = ?", id).
			First(&existing).
			Error; err != nil {
			return fmt.Errorf("failed to find existing key version %s: %w", id, err)
		}

		// If the provided key is already active, do not attempt to re-activate it.
		if existing.IsActive {
			return nil
		}

		// Disable old actives.
		if err := tx.
			Model(&TokenSigningKey{}).
			Where("is_active = ?", true).
			Update("is_active", false).
			Error; err != nil {
			return fmt.Errorf("failed to deactivate old key versions: %w", err)
		}

		// Enable new active version.
		existing.IsActive = true
		if err := tx.Save(existing).Error; err != nil {
			return fmt.Errorf("failed to activate key version: %w", err)
		}

		// Audit.
		audit := BuildAuditEntry(actor, "activated token signing key version", &existing, 0)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	})
}

// RotateTokenSigningKey creates a new key in the upstream kms provider. If
// creating the upstream key fails, an error is returned. If the upstream key is
// successfully created, a new TokenSigningKey record is created in the database
// (not yet active). Finally, the new key is set as the active key.
func (db *Database) RotateTokenSigningKey(ctx context.Context, kms keys.KeyVersionCreator, parent string, actor Auditable) (*TokenSigningKey, error) {
	result, err := kms.CreateKeyVersion(ctx, parent)
	if err != nil {
		return nil, fmt.Errorf("failed to create key version in upstream kms: %w", err)
	}

	key := &TokenSigningKey{KeyVersionID: result}
	if err := db.SaveTokenSigningKey(key, actor); err != nil {
		return nil, fmt.Errorf("failed to save token signing key: %w", err)
	}

	if err := db.ActivateTokenSigningKey(key.ID, actor); err != nil {
		return nil, fmt.Errorf("failed to activate token signing key: %w", err)
	}

	// Go lookup the key. Note that we don't just return the key here, because it
	// might have mutated state from other operations. This ensures the result is
	// fresh from the database upon return.
	return db.FindTokenSigningKey(key.ID)
}

// PurgeTokenSigningKeys will delete token signing keys that have been rotated
// more than the provided max age.
func (db *Database) PurgeTokenSigningKeys(ctx context.Context, kms keys.KeyVersionDestroyer, maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	rotatedBefore := time.Now().UTC().Add(maxAge)

	// Select all keys currently targeted for deletion.
	var keys []*TokenSigningKey
	if err := db.db.
		Unscoped().
		Where("is_active IS FALSE AND updated_at IS NOT NULL AND updated_at < ?", rotatedBefore).
		Find(&keys).
		Error; err != nil {
		return 0, fmt.Errorf("failed to find existing keys: %w", err)
	}

	// Iterate over each key and attempt to delete.
	for _, key := range keys {
		// Destroy upstream.
		if err := kms.DestroyKeyVersion(ctx, key.KeyVersionID); err != nil {
			return 0, fmt.Errorf("failed to destroy key version %q: %w", key.KeyVersionID, err)
		}

		// Delete from database.
		if err := db.db.Unscoped().Delete(key).Error; err != nil {
			return 0, fmt.Errorf("failed to delete key version %d: %w", key.ID, err)
		}
	}

	return int64(len(keys)), nil
}
