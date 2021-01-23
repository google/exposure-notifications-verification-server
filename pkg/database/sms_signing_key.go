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
