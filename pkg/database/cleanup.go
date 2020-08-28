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
	"errors"
	"time"

	"github.com/jinzhu/gorm"
)

var (
	ErrCleanupWrongGeneration = errors.New("cleanup wrong generation")
)

var CleanupName = "cleanup"

type CleanupStatus struct {
	gorm.Model
	Type       string `gorm:"type:varchar(50);unique_index"`
	Generation uint
	NotBefore  time.Time
}

// TableName sets the CleanupStatus table name
func (CleanupStatus) TableName() string {
	return "cleanup_statuses"
}

// CreateCleanup is used to create a new 'cleanup' type/row in the database.
func (db *Database) CreateCleanup(cType string) (*CleanupStatus, error) {
	cstat := &CleanupStatus{
		Type:       cType,
		Generation: 1,
		NotBefore:  time.Now().UTC(),
	}
	if err := db.db.
		Set("gorm:insert_option", "ON CONFLICT (type) DO NOTHING").
		FirstOrCreate(cstat).
		Error; err != nil {
		return nil, err
	}
	return cstat, nil
}

// FindCleanupStatus looks up the current cleanup state in the database by cleanup type.
func (db *Database) FindCleanupStatus(cType string) (*CleanupStatus, error) {
	var cstat CleanupStatus
	if err := db.db.Where("type = ?", cType).First(&cstat).Error; err != nil {
		return nil, err
	}
	return &cstat, nil
}

// ClaimCleanup attempts to obtain a lock for the specified `lockTime` so that that type of
// cleanup can be perofmed exclusively by the owner.
func (db *Database) ClaimCleanup(current *CleanupStatus, lockTime time.Duration) (*CleanupStatus, error) {
	var cstat CleanupStatus
	err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Where("type = ?", current.Type).
			First(&cstat).
			Error; err != nil {
			return err
		}
		if cstat.Generation != current.Generation {
			return ErrCleanupWrongGeneration
		}

		cstat.Generation++
		cstat.NotBefore = time.Now().UTC().Add(lockTime)
		return tx.Save(&cstat).Error
	})
	if err != nil {
		return nil, err
	}
	return &cstat, nil
}
