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
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
)

var ErrWrongGeneration = errors.New("wrong generation")

// LockStatus represents a distributed lock that spaces operations out.
// These are only self expring locks (NotBefore) and are not explicitly
// released.
type LockStatus struct {
	gorm.Model
	Type       string `gorm:"type:varchar(50);unique_index"`
	Generation uint
	NotBefore  time.Time
}

// TryLock is used to ensure that only one app sync process runs per AppSyncPeriod duration.
func (db *Database) TryLock(ctx context.Context, lockName string, lockDuration time.Duration) (bool, error) {
	cStat, err := db.CreateLock(lockName)
	if err != nil {
		return false, fmt.Errorf("failed to create %s lock: %w", lockName, err)
	}

	if cStat.NotBefore.After(time.Now().UTC()) {
		return false, nil
	}

	// Attempt to advance the generation.
	if _, err = db.ClaimLock(cStat, lockDuration); err != nil {
		return false, fmt.Errorf("failed to claim %s lock: %w", lockName, err)
	}
	return true, nil
}

// CreateLock is used to create a new 'cleanup' type/row in the database.
func (db *Database) CreateLock(cType string) (*LockStatus, error) {
	var status LockStatus

	sql := `INSERT INTO lock_statuses (type, generation, not_before)
		VALUES ($1, $2, $3)
		ON CONFLICT (type) DO UPDATE SET type = EXCLUDED.type
		RETURNING *`

	now := time.Now().UTC()
	if err := db.db.
		Raw(sql, cType, 1, now).
		Scan(&status).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

// FindLockStatus looks up the current cleanup state in the database by cleanup type.
func (db *Database) FindLockStatus(cType string) (*LockStatus, error) {
	var status LockStatus
	if err := db.db.Where("type = ?", cType).First(&status).Error; err != nil {
		return nil, err
	}
	return &status, nil
}

// ClaimLock attempts to obtain a lock for the specified `lockTime` so that
// that type of cleanup can be performed exclusively by the owner.
func (db *Database) ClaimLock(current *LockStatus, lockTime time.Duration) (*LockStatus, error) {
	var status LockStatus
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Model(&LockStatus{}).
			Where("type = ?", current.Type).
			First(&status).
			Error; err != nil {
			return err
		}
		if status.Generation != current.Generation {
			return ErrWrongGeneration
		}

		status.Generation++
		status.NotBefore = time.Now().UTC().Add(lockTime)
		return tx.Save(&status).Error
	}); err != nil {
		return nil, err
	}
	return &status, nil
}
