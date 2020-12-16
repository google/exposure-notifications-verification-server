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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
)

// Membership represents a user's membership in a realm.
type Membership struct {
	gorm.Model
	Errorable

	UserID uint
	User   *User

	RealmID uint
	Realm   *Realm

	DefaultSMSTemplateLabel string `gorm:"type:varchar(255);"`

	Permissions rbac.Permission
}

func (db *Database) SaveMembership(m *Membership, actor Auditable) error {
	if m == nil {
		return fmt.Errorf("provided membership is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing Membership
		if err := tx.
			Model(&Realm{}).
			Where("id = ?", m.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing membership")
		}

		// Brand new realm?
		if existing.ID == 0 {
			return fmt.Errorf("memberships may not be directly created - need to addToRealm")
		}

		if err := existing.AfterFind(); err != nil {
			return err
		}

		// Save the realm
		if err := tx.Save(m).Error; err != nil {
			return fmt.Errorf("failed to save membership: %w", err)
		}

		if existing.DefaultSMSTemplateLabel != m.DefaultSMSTemplateLabel {
			audit := BuildAuditEntry(actor, "updated membership default template", m.User, m.ID)
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

	if msgs := m.ErrorMessages(); len(msgs) > 0 {
		return fmt.Errorf("lookup failed: %s", strings.Join(msgs, ", "))
	}
	return nil
}

// Can returns true if the membership has the checked permission on the realm,
// false otherwise.
func (m *Membership) Can(p rbac.Permission) bool {
	if m == nil {
		return false
	}
	return rbac.Can(m.Permissions, p)
}
