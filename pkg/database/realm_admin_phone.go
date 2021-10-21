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

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/jinzhu/gorm"
)

var _ Auditable = (*AuthorizedApp)(nil)

// RealmAdminPhone represends a destination phone number for Realm Admin
// alerts / updates
type RealmAdminPhone struct {
	gorm.Model
	Errorable

	// RealmAdminPhones belong to exactly one realm.
	RealmID uint

	// Name is the name of the user who the phone belongs to.
	Name string `gorm:"type:text"`

	// PhoneNumber is the destination phone number to send messages to.
	// E.164 format telephone number
	PhoneNumber string `gorm:"type:text"`
}

// BeforeSave runs validations. If there are errors, the save fails.
func (rap *RealmAdminPhone) BeforeSave(tx *gorm.DB) error {
	rap.Name = project.TrimSpace(rap.Name)
	rap.PhoneNumber = project.TrimSpace(rap.PhoneNumber)

	if rap.Name == "" {
		rap.AddError("name", "cannot be blank")
	}

	if rap.PhoneNumber == "" {
		rap.AddError("phoneNumber", "cannot be blank")
	}

	return rap.ErrorOrNil()
}

func (rap *RealmAdminPhone) AuditID() string {
	return fmt.Sprintf("realm_admin_phone:%d", rap.ID)
}

func (rap *RealmAdminPhone) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", rap.Name, rap.PhoneNumber)
}

// SaveRealmAdminPhone saves the realm admin phone number
func (db *Database) SaveRealmAdminPhone(rap *RealmAdminPhone, actor Auditable) error {
	if rap == nil {
		return fmt.Errorf("provided realm admin phone is nil")
	}
	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing RealmAdminPhone
		if err := tx.
			Unscoped().
			Model(&RealmAdminPhone{}).
			Where("id = ?", rap.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing mobile app: %w", err)
		}

		// Save the app
		if err := tx.Unscoped().Save(rap).Error; err != nil {
			if IsUniqueViolation(err, "uix_admin_phone_name_realm") {
				rap.AddError("name", "must be unique")
				return ErrValidationFailed
			}
			return err
		}

		// Brand new app?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created realm admin phone", rap, rap.RealmID)
			audits = append(audits, audit)
		} else {
			if existing.Name != rap.Name {
				audit := BuildAuditEntry(actor, "updated realm admin phone name", rap, rap.RealmID)
				audit.Diff = stringDiff(existing.Name, rap.Name)
				audits = append(audits, audit)
			}

			if existing.PhoneNumber != rap.PhoneNumber {
				audit := BuildAuditEntry(actor, "updated realm admin phone value", rap, rap.RealmID)
				audit.Diff = stringDiff(existing.PhoneNumber, rap.PhoneNumber)
				audits = append(audits, audit)
			}

			if existing.DeletedAt != rap.DeletedAt {
				audit := BuildAuditEntry(actor, "updated realm admin phone enabled", rap, rap.RealmID)
				audit.Diff = boolDiff(existing.DeletedAt == nil, rap.DeletedAt == nil)
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
