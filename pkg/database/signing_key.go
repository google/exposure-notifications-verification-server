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
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
)

var _ RealmManagedKey = (*SigningKey)(nil)

// SigningKey represents a reference to a KMS backed signing key
// version for verification certificate signing.
type SigningKey struct {
	gorm.Model
	Errorable

	// A signing key belongs to exactly one realm.
	RealmID uint `gorm:"index:realm"`

	// Reference to an exact version of a key in the KMS
	KeyID  string
	Active bool
}

// AuditID is how the signing key is stored in the audit entry.
func (s *SigningKey) AuditID() string {
	return fmt.Sprintf("certificate_signing_key:%d", s.ID)
}

// AuditDisplay is how the signing key will be displayed in audit entries.
func (s *SigningKey) AuditDisplay() string {
	return fmt.Sprintf("certificate signing key (%s)", s.GetKID())
}

// GetKID returns the 'kid' field value to use in signing JWTs.
func (s *SigningKey) GetKID() string {
	return fmt.Sprintf("r%dv%d", s.RealmID, s.ID)
}

func (s *SigningKey) ManagedKeyID() string {
	return s.KeyID
}

func (s *SigningKey) IsActive() bool {
	return s.Active
}

func (s *SigningKey) SetRealmID(id uint) {
	s.RealmID = id
}

func (s *SigningKey) SetManagedKeyID(keyID string) {
	s.KeyID = keyID
}

func (s *SigningKey) SetActive(active bool) {
	s.Active = active
}

func (s *SigningKey) Table() string {
	return "signing_keys"
}

func (s *SigningKey) Purpose() string {
	return "certificate"
}

// PurgeSigningKeys will purge soft deleted keys that have been soft deleted for maxAge duration.
func (db *Database) PurgeSigningKeys(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)

	result := db.db.Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", deleteBefore).
		Delete(&SigningKey{})
	return result.RowsAffected, result.Error
}
