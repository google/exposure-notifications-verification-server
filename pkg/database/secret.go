// Copyright 2021 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/jinzhu/gorm"
)

// SecretType represents a secret type.
type SecretType string

const (
	SecretTypeAPIKeyDatabaseHMAC           = SecretType("api_key_database_hmac")
	SecretTypeAPIKeySignatureHMAC          = SecretType("api_key_signature_hmac")
	SecretTypeCookieKeys                   = SecretType("cookie_keys")
	SecretTypePhoneNumberDatabaseHMAC      = SecretType("phone_number_database_hmac")
	SecretTypeVerificationCodeDatabaseHMAC = SecretType("verification_code_database_hmac")
)

var _ Auditable = (*Secret)(nil)

// Secret represents the reference to a secret in an upstream secret manager. It
// exists to facilitate rotation and auditing.
type Secret struct {
	Errorable

	// ID is the primary key of the secret.
	ID uint

	// Type is the type of secret.
	Type SecretType

	// Reference is the pointer to the secret in the secret manager.
	Reference string

	// Active is a boolean indicating whether this secret is active.
	Active bool

	// CreatedAt, UpdatedAt, and DeletedAt are the timestamps.
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (s *Secret) AuditID() string {
	return fmt.Sprintf("secret:%d", s.ID)
}

func (s *Secret) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", s.Type, s.Reference)
}

// BeforeSave runs validations. If there are errors, the save fails.
func (s *Secret) BeforeSave(tx *gorm.DB) error {
	s.Reference = project.TrimSpace(s.Reference)

	if s.Reference == "" {
		s.AddError("reference", "cannot be blank")
	}

	if err := s.ErrorOrNil(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.Join(s.ErrorMessages(), ", "))
	}
	return nil
}

// FindSecret gets a specific secret by its database ID.
func (db *Database) FindSecret(id interface{}) (*Secret, error) {
	var secret Secret
	if err := db.db.
		Model(&Secret{}).
		Where("id = ?", id).
		First(&secret).
		Error; err != nil {
		return nil, fmt.Errorf("failed to find secret %v: %w", id, err)
	}
	return &secret, nil
}

// ListSecrets lists all secrets in the database.
func (db *Database) ListSecrets(scopes ...Scope) ([]*Secret, error) {
	var secrets []*Secret
	if err := db.db.
		Scopes(scopes...).
		Model(&Secret{}).
		Find(&secrets).
		Error; err != nil {
		if IsNotFound(err) {
			return secrets, nil
		}
	}
	return secrets, nil
}

// ListSecretsForType lists all secrets for the given type, ordered by their
// creation date, but with inactive secrets (ones not ready to be used as
// primary) at the end of the list, allowing for propagation over time.
func (db *Database) ListSecretsForType(typ SecretType, scopes ...Scope) ([]*Secret, error) {
	scopes = append(scopes, InConsumableSecretOrder())

	var secrets []*Secret
	if err := db.db.
		Scopes(scopes...).
		Model(&Secret{}).
		Where("secrets.type = ?", typ).
		Find(&secrets).
		Error; err != nil {
		if IsNotFound(err) {
			return secrets, nil
		}
	}
	return secrets, nil
}

// ActivateSecrets activates all secrets that are not currently activate but
// have been created for since the provided timestamp.
func (db *Database) ActivateSecrets(typ SecretType, since time.Time) error {
	if err := db.db.
		Model(&Secret{}).
		Where("active IS FALSE AND type = ? AND created_at < ?", typ, since).
		UpdateColumn("active", true).
		Error; err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to activate secrets: %w", err)
	}
	return nil
}

// SaveSecret creates or updates the secret.
func (db *Database) SaveSecret(s *Secret, actor Auditable) error {
	if s == nil {
		return fmt.Errorf("provided secret is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing Secret
		if err := tx.
			Unscoped().
			Model(&Secret{}).
			Where("id = ?", s.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing secret: %w", err)
		}

		// Save the record
		if err := tx.Unscoped().Save(s).Error; err != nil {
			return err
		}

		// New record?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created secret", s, 0)
			audits = append(audits, audit)
		} else {
			if existing.Type != s.Type {
				audit := BuildAuditEntry(actor, "updated secret type", s, 0)
				audit.Diff = stringDiff(string(existing.Type), string(s.Type))
				audits = append(audits, audit)
			}

			if existing.Reference != s.Reference {
				audit := BuildAuditEntry(actor, "updated secret reference", s, 0)
				audit.Diff = stringDiff(existing.Reference, s.Reference)
				audits = append(audits, audit)
			}

			if existing.Active != s.Active {
				audit := BuildAuditEntry(actor, "updated secret active status", s, 0)
				audit.Diff = boolDiff(existing.Active, s.Active)
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

// DeleteSecret performs a soft delete on the provided secret.
func (db *Database) DeleteSecret(s *Secret, actor Auditable) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(s).Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to delete secret: %w", err)
		}

		audit := BuildAuditEntry(actor, "marked secret for deletion", s, 0)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	})
}

// PurgeSecret deletes the secret for real.
func (db *Database) PurgeSecret(s *Secret, actor Auditable) error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(s).Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to purge secret: %w", err)
		}

		audit := BuildAuditEntry(actor, "purged secret", s, 0)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	})
}
