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
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
)

const (
	modelerLockTime = 15 * time.Minute
)

type ModelerStatus struct {
	ID        uint `gorm:"primary_key"`
	NotBefore time.Time
}

// ClaimModelerStatus attempts to claim the modeler status lock. This acquires a
// 15min lock on the table to prevent concurrent modifications over
// subscription. If the function returns nil, it successfully claimed the lock.
// Otherwise, lock acqusition was not successful and the caller should NOT
// continue processing.
func (db *Database) ClaimModelerStatus() error {
	return db.db.Transaction(func(tx *gorm.DB) error {
		var r ModelerStatus
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			First(&r).
			Error; err != nil {
			return err
		}

		if time.Now().UTC().Unix() < r.NotBefore.Unix() {
			return fmt.Errorf("too soon (wait until %s)", r.NotBefore.Format(time.RFC3339))
		}

		r.NotBefore = time.Now().UTC().Add(modelerLockTime)
		return tx.Save(&r).Error
	})
}
