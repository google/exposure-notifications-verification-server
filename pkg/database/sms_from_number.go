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
	"strings"

	"github.com/jinzhu/gorm"
)

// SMSFromNumber represents a source number which can send SMS messages. The
// table only contains the system SMS from numbers.
type SMSFromNumber struct {
	Errorable

	ID    uint   `gorm:"primary_key;" json:"id,omitempty"`
	Label string `gorm:"column:label;" json:"label"`
	Value string `gorm:"column:value;" json:"value"`
}

func (s *SMSFromNumber) BeforeSave(tx *gorm.DB) error {
	if s.Label == "" {
		s.AddError("label", "cannot be blank")
	}

	if s.Value == "" {
		s.AddError("value", "cannot be blank")
	}

	if len(s.Errors()) > 0 {
		return fmt.Errorf("sms from number validation failed: %s", strings.Join(s.ErrorMessages(), ", "))
	}
	return nil
}

// SMSFromNumbers returns the list of SMS from numbers in the system.
func (db *Database) SMSFromNumbers(scopes ...Scope) ([]*SMSFromNumber, error) {
	var numbers []*SMSFromNumber

	if err := db.db.
		Model(&SMSFromNumber{}).
		Scopes(scopes...).
		Order("label ASC").
		Find(&numbers).
		Error; err != nil {
		if IsNotFound(err) {
			return numbers, nil
		}
		return nil, err
	}
	return numbers, nil
}

// FindSMSFromNumber finds the given SMS from number by ID.
func (db *Database) FindSMSFromNumber(id interface{}) (*SMSFromNumber, error) {
	var number SMSFromNumber
	if err := db.db.
		Model(&SMSFromNumber{}).
		Where("id = ?", id).
		First(&number).
		Error; err != nil {
		return nil, err
	}
	return &number, nil
}

// CreateOrUpdateSMSFromNumbers takes the list of SMS numbers and creates new
// records, updates existing records, and deletes records that are not present
// in the list.
func (db *Database) CreateOrUpdateSMSFromNumbers(numbers []*SMSFromNumber) error {
	ids := make([]uint, 0, len(numbers))

	return db.db.Transaction(func(tx *gorm.DB) error {
		for _, number := range numbers {
			if number.ID == 0 {
				if err := tx.Model(&SMSFromNumber{}).Create(number).Error; err != nil {
					return fmt.Errorf("failed to create %s: %w", number.Label, err)
				}
			} else {
				if err := tx.Model(&SMSFromNumber{}).Update(number).Error; err != nil {
					return fmt.Errorf("failed to update %s: %w", number.Label, err)
				}
			}

			ids = append(ids, number.ID)
		}

		del := tx.Unscoped()
		if len(ids) > 0 {
			del = del.Where("id NOT IN (?)", ids)
		}
		if err := del.Delete(&SMSFromNumber{}).Error; err != nil {
			return fmt.Errorf("failed to delete old sms numbers: %w", err)
		}

		return nil
	})
}
