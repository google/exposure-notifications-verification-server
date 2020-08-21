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

import "github.com/jinzhu/gorm"

// SigningKey represents a reference to a KMS backed signing key
// for verification certificate signing.
type SigningKey struct {
	gorm.Model
	Errorable

	// A signing key belongs to exactly one realm.
	RealmID uint `gorm:"index:addr"`

	KeyID string
}

func (s *SigningKey) Delete(db *Database) error {
	return db.db.Delete(s).Error
}

func (db *Database) SaveSigningKey(s *SigningKey) error {
	if s.Model.ID == 0 {
		return db.db.Create(s).Error
	}
	return db.db.Save(s).Error
}
