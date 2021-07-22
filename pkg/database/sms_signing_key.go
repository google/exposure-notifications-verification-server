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
	"time"

	"github.com/jinzhu/gorm"
)

var _ RealmManagedKey = (*SMSSigningKey)(nil)

// SMSSigningKey represents a reference to a KMS backed signing key
// version for SMS payload signing.
type SMSSigningKey struct {
	gorm.Model
	Errorable

	// A signing key belongs to exactly one realm.
	RealmID uint

	// Reference to an exact version of a key in the KMS
	KeyID  string
	Active bool
}

// FindSMSSigningKey finds an SMS signing key by the provided database id.
func (db *Database) FindSMSSigningKey(id interface{}) (*SMSSigningKey, error) {
	var key SMSSigningKey
	if err := db.db.
		Model(&SMSSigningKey{}).
		Where("id = ?", id).
		First(&key).
		Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// AuditID is how the signing key is stored in the audit entry.
func (s *SMSSigningKey) AuditID() string {
	return fmt.Sprintf("sms_signing_key:%d", s.ID)
}

// AuditDisplay is how the signing key will be displayed in audit entries.
func (s *SMSSigningKey) AuditDisplay() string {
	return fmt.Sprintf("sms signing key (%s)", s.GetKID())
}

// GetKID returns the 'kid' field value to use in signing JWTs.
func (s *SMSSigningKey) GetKID() string {
	return fmt.Sprintf("r%dv%dsms", s.RealmID, s.ID)
}

func (s *SMSSigningKey) ManagedKeyID() string {
	return s.KeyID
}

func (s *SMSSigningKey) IsActive() bool {
	return s.Active
}

func (s *SMSSigningKey) SetRealmID(id uint) {
	s.RealmID = id
}

func (s *SMSSigningKey) SetManagedKeyID(keyID string) {
	s.KeyID = keyID
}

func (s *SMSSigningKey) SetActive(active bool) {
	s.Active = active
}

func (s *SMSSigningKey) Table() string {
	return "sms_signing_keys"
}

func (s *SMSSigningKey) Purpose() string {
	return "SMS"
}

// PurgeSMSSigningKeys will purge soft deleted keys that have been soft deleted for maxAge duration.
func (db *Database) PurgeSMSSigningKeys(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)

	result := db.db.Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", deleteBefore).
		Delete(&SMSSigningKey{})
	return result.RowsAffected, result.Error
}
