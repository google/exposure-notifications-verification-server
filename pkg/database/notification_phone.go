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

var _ Auditable = (*NotificationPhone)(nil)

// NotificationPhone represends a destination phone number for Realm Admin
// alerts / updates
type NotificationPhone struct {
	gorm.Model
	Errorable

	// RealmAdminPhones belong to exactly one realm.
	RealmID uint `gorm:"column:realm_id; type: integer;"`

	// Name is the name of the user who the phone belongs to.
	Name string `gorm:"column:name; type:text;"`

	// PhoneNumber is the destination phone number to send messages to.
	// E.164 format telephone number
	PhoneNumber string `gorm:"column:phone_number; type:text;"`

	// Populated to attempt to format phone number as E164
	smsCountry string `gorm:"-"`
}

// BeforeSave runs validations. If there are errors, the save fails.
func (rap *NotificationPhone) BeforeSave(tx *gorm.DB) error {
	rap.Name = project.TrimSpace(rap.Name)
	rap.PhoneNumber = project.TrimSpace(rap.PhoneNumber)

	if rap.Name == "" {
		rap.AddError("name", "cannot be blank")
	}

	if rap.PhoneNumber == "" {
		rap.AddError("phone_number", "cannot be blank")
	} else {
		canonicalPhone, err := project.CanonicalPhoneNumber(rap.PhoneNumber, rap.smsCountry)
		if err != nil {
			rap.AddError("phone_number", fmt.Sprintf("invalid format: %q", err.Error()))
		} else {
			rap.PhoneNumber = canonicalPhone
		}
	}

	return rap.ErrorOrNil()
}

func (rap *NotificationPhone) AuditID() string {
	return fmt.Sprintf("realm_admin_phone:%d", rap.ID)
}

func (rap *NotificationPhone) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", rap.Name, rap.PhoneNumber)
}

// SaveRealmAdminPhone saves the realm admin phone number
func (db *Database) SaveRealmAdminPhone(realm *Realm, rap *NotificationPhone, actor Auditable) error {
	if rap == nil {
		return fmt.Errorf("provided realm admin phone is nil")
	}
	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry
		rap.RealmID = realm.ID
		rap.smsCountry = realm.SMSCountry

		var existing NotificationPhone
		if err := tx.
			Unscoped().
			Model(&NotificationPhone{}).
			Where("id = ?", rap.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing mobile app: %w", err)
		}

		// Save the app
		if err := tx.Unscoped().Save(rap).Error; err != nil {
			if IsUniqueViolation(err, "uix_notification_phone_name_realm") {
				rap.AddError("name", "must be unique")
				return ErrValidationFailed
			}
			if IsUniqueViolation(err, "uix_notification_phone_number_realm") {
				rap.AddError("phone_number", "must be unique")
				return ErrValidationFailed
			}
			return err
		}

		// Brand new app?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created realm notification phone number", rap, rap.RealmID)
			audits = append(audits, audit)
		} else {
			if existing.Name != rap.Name {
				audit := BuildAuditEntry(actor, "updated realm notification phone name", rap, rap.RealmID)
				audit.Diff = stringDiff(existing.Name, rap.Name)
				audits = append(audits, audit)
			}

			if existing.PhoneNumber != rap.PhoneNumber {
				audit := BuildAuditEntry(actor, "updated realm notification phone value", rap, rap.RealmID)
				audit.Diff = stringDiff(existing.PhoneNumber, rap.PhoneNumber)
				audits = append(audits, audit)
			}

			if existing.DeletedAt != rap.DeletedAt {
				audit := BuildAuditEntry(actor, "updated realm notification phone enabled", rap, rap.RealmID)
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
