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

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
)

// Membership represents a user's membership in a realm.
type Membership struct {
	Errorable

	UserID uint
	User   *User

	RealmID uint
	Realm   *Realm

	// DefaultSMSTemplateLabel is the label of realm.SMSTextAlternateTemplates or "Default SMS template"
	// that the user last used to issue codes. This helps the UI remember the default user preference.
	// Note: This label may not exist if it has been deleted or modified on the realm.
	DefaultSMSTemplateLabel string `gorm:"type:varchar(255);"`

	Permissions rbac.Permission

	// CreatedAt is when the user was added to the realm. UpdatedAt is when the
	// user's permissions were last updated. Note that UpdatedAt only applies to
	// the membership's fields, not the user fields (e.g. email, name).
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SaveMembership saves the membership details. Should have a userID and a
// realmID to identify it.
func (db *Database) SaveMembership(m *Membership, actor Auditable) error {
	if m == nil {
		return fmt.Errorf("provided membership is nil")
	}

	if actor == nil {
		return ErrMissingActor
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing Membership
		if err := tx.
			Model(&Membership{}).
			Where("user_id = ? AND realm_id = ?", m.UserID, m.RealmID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing membership: %w", err)
		}

		// Save the membership
		if existing.UserID != 0 {
			if err := tx.Model(&existing).Update(m).Error; err != nil {
				return fmt.Errorf("failed to update membership: %w", err)
			}
		} else {
			if err := tx.Model(&Membership{}).Create(m).Error; err != nil {
				return fmt.Errorf("failed to create membership: %w", err)
			}
		}

		if existing.DefaultSMSTemplateLabel != m.DefaultSMSTemplateLabel {
			audit := BuildAuditEntry(actor, "updated membership default template", m.User, m.RealmID)
			audit.Diff = stringDiff(existing.DefaultSMSTemplateLabel, m.DefaultSMSTemplateLabel)
			audits = append(audits, audit)
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

// AfterFind does a sanity check to ensure the User and Realm properties were
// preloaded and the referenced values exist.
func (m *Membership) AfterFind() error {
	if m.User == nil {
		m.AddError("user", "does not exist")
	}

	if m.Realm == nil {
		m.AddError("realm", "does not exist")
	}

	return m.ErrorOrNil()
}

// PurgeOrphanedMemberships will delete memberships that have no permissions. This
// exists to fix a bug that existed since 2021-08-24. It should be safe to
// remove this code in 0.40.0+, but it's also not hurting anything to stick
// around.
func (db *Database) PurgeOrphanedMemberships() (int64, error) {
	rtn := db.db.Unscoped().
		Where("memberships.permissions = 0").
		Delete(&Membership{})
	return rtn.RowsAffected, rtn.Error
}

// Can returns true if the membership has the checked permission on the realm,
// false otherwise.
func (m *Membership) Can(p rbac.Permission) bool {
	if m == nil {
		return false
	}
	return rbac.Can(m.Permissions, p)
}

// Cannot returns the opposite of Can
func (m *Membership) Cannot(p rbac.Permission) bool {
	return !m.Can(p)
}
