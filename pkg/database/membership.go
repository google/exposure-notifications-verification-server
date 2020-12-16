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

	DefaultSMSTemplateLabel string `gorm:"type:varchar(255);"`

	Permissions rbac.Permission
}

// SaveMembership saves the membership details. Should have a userID and a realmID to identify it.
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
			Model(&Membership{}).
			Where("user_id = ? AND realm_id = ?", m.UserID, m.RealmID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing membership")
		}

		if err := existing.AfterFind(); err != nil {
			return err
		}

		// Save the realm
		if err := tx.Update(m).Error; err != nil {
			return fmt.Errorf("failed to save membership: %w", err)
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

// ListMemberships lists the memberships for this user. Use
// ListMembershipsCached where possible.
func (u *User) ListMemberships(db *Database) ([]*Membership, error) {
	var memberships []*Membership

	if err := db.db.
		Preload("Realm").
		Preload("User").
		Model(&Membership{}).
		Where("user_id = ?", u.ID).
		Joins("JOIN realms ON realms.id = memberships.realm_id").
		Order("realms.name").
		Find(&memberships).
		Error; err != nil {
		if IsNotFound(err) {
			return memberships, nil
		}
		return nil, err
	}
	return memberships, nil
}

// FindMembership finds the corresponding membership for the given realm ID, if
// one exists. If not does not exist, an error is returned that satisfies
// IsNotFound.
func (u *User) FindMembership(db *Database, realmID interface{}) (*Membership, error) {
	var membership Membership
	if err := db.db.
		Model(&Membership{}).
		Preload("Realm").
		Preload("User").
		Where("user_id = ? AND realm_id = ?", u.ID, realmID).
		First(&membership).
		Error; err != nil {
		return nil, err
	}
	return &membership, nil
}

// AddToRealm adds the current user to the realm with the given permissions. If
// a record already exists, the permissions are overwritten with the new
// permissions.
func (u *User) AddToRealm(db *Database, r *Realm, permissions rbac.Permission, actor Auditable) error {
	if actor == nil {
		return fmt.Errorf("auditable actor cannot be nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var existing Membership
		if err := tx.
			Model(&Membership{}).
			Where("user_id = ? AND realm_id = ?", u.ID, r.ID).
			First(&existing).
			Error; err != nil {
			if !IsNotFound(err) {
				return err
			}
		}

		conflict := fmt.Sprintf(`ON CONFLICT (user_id, realm_id) DO UPDATE
			SET permissions = %d`, permissions)
		if err := tx.
			Set("gorm:insert_option", conflict).
			Model(&Membership{}).
			Create(&Membership{
				UserID:      u.ID,
				RealmID:     r.ID,
				Permissions: permissions,
			}).
			Error; err != nil {
			return err
		}

		// Brand new member?
		if existing.UserID == 0 {
			audit := BuildAuditEntry(actor, "added user to realm", u, r.ID)
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audit: %w", err)
			}
		}

		// Audit if permissions were changed.
		if old, new := existing.Permissions, permissions; old != new {
			audit := BuildAuditEntry(actor, "updated user permissions", u, r.ID)
			audit.Diff = stringSliceDiff(rbac.PermissionNames(old), rbac.PermissionNames(new))
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audit: %w", err)
			}
		}

		// Cascade updated_at on user
		if err := tx.
			Model(&User{}).
			Where("id = ?", u.ID).
			UpdateColumn("updated_at", time.Now().UTC()).
			Error; err != nil {
			return fmt.Errorf("failed to update user updated_at: %w", err)
		}

		return nil
	})
}

// DeleteFromRealm removes this user from the given realm. If the user does not
// exist in the realm, no action is taken.
func (u *User) DeleteFromRealm(db *Database, r *Realm, actor Auditable) error {
	if actor == nil {
		return fmt.Errorf("auditable actor cannot be nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Model(&Membership{}).
			Where("user_id = ? AND realm_id = ?", u.ID, r.ID).
			Delete(&Membership{
				UserID:  u.ID,
				RealmID: r.ID,
			}).
			Error; err != nil {
			if !IsNotFound(err) {
				return err
			}
		}

		// Generate audit
		audit := BuildAuditEntry(actor, "removed user from realm", u, r.ID)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audit: %w", err)
		}

		// Cascade updated_at on user
		if err := tx.
			Model(&User{}).
			Where("id = ?", u.ID).
			UpdateColumn("updated_at", time.Now().UTC()).
			Error; err != nil {
			return fmt.Errorf("failed to update user updated_at: %w", err)
		}

		return nil
	})
}
