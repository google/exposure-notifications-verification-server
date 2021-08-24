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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
)

// BulkPermissionAction is the permission action to take.
type BulkPermissionAction uint8

const (
	_ BulkPermissionAction = iota
	BulkPermissionActionAdd
	BulkPermissionActionRemove
)

// BulkPermission represents a bulk permission operation. This is not actually a
// table in the database.
type BulkPermission struct {
	Errorable

	RealmID     uint
	UserIDs     []uint
	Permissions rbac.Permission
	Action      BulkPermissionAction
}

// Apply converges the bulk operation. If a user isn't in the realm, no action
// is taken.
//
// For add operations, if the user already has the permission, no action is
// taken. For remove operations, if the user does not have the permission, no
// action is taken.
//
// Other permissions not in the list are unchanged.
func (b *BulkPermission) Apply(db *Database, actor Auditable) error {
	// Bulk update is all-or-nothing, so do everything in a transaction.
	return db.db.Transaction(func(tx *gorm.DB) error {
		// Fetch all current memberships - this is required for re-building implied
		// permissions and auditing.
		var memberships []*Membership
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Model(&Membership{}).
			Where("realm_id = ?", b.RealmID).
			Where("user_id IN (?)", b.UserIDs).
			Find(&memberships).
			Error; err != nil {
			if IsNotFound(err) {
				return nil
			}
			return err
		}

		// Process each membership individually.
		for _, membership := range memberships {
			// Users cannot update their own permissions.
			if user, ok := actor.(*User); ok && membership.UserID == user.ID {
				continue
			}

			// Compute new permissions.
			newPerms, existingPerms := membership.Permissions, membership.Permissions
			switch b.Action {
			case BulkPermissionActionAdd:
				newPerms = newPerms | b.Permissions
			case BulkPermissionActionRemove:
				newPerms = newPerms &^ b.Permissions

				// Re-compute implied permissions. This handles an edge case where
				// someone removes an implied permission but not the implying
				// permission. For example, if someone bulk-removed a Read but not
				// Write, memberships with Write should still retain read because its
				// implied.
				//
				// There's also a weird security edge case here in that we do not check
				// if the actor has this permission. In this case, the membership
				// already had the permission, so even if the actor doesn't have said
				// permission, it's not privilege escalation.
				newPerms = rbac.AddImplied(newPerms)
			}

			// It's possible that no permissions have changed, in which case we don't
			// need to save the record or create an audit entry.
			if newPerms == existingPerms {
				continue
			}

			if newPerms == 0 {
				if err := tx.
					Unscoped().
					Model(&Membership{}).
					Where("realm_id = ?", membership.RealmID).
					Where("user_id = ?", membership.UserID).
					Delete(&Membership{
						RealmID: membership.RealmID,
						UserID:  membership.UserID,
					}).
					Error; err != nil {
					return fmt.Errorf("failed to delete membership: %w", err)
				}

				// Generate audit
				audit := BuildAuditEntry(actor, "removed user from realm", membership.User, membership.RealmID)
				if err := tx.Save(audit).Error; err != nil {
					return fmt.Errorf("failed to save audit: %w", err)
				}
			} else {
				// Save the membership.
				if err := tx.
					Model(&Membership{}).
					Where("realm_id = ?", membership.RealmID).
					Where("user_id = ?", membership.UserID).
					Update("permissions", newPerms).
					Error; err != nil {
					return fmt.Errorf("failed to save membership: %w", err)
				}

				// Audit if permissions were changed.
				audit := BuildAuditEntry(actor, "updated user permissions", actor, membership.RealmID)
				audit.Diff = stringSliceDiff(rbac.PermissionNames(existingPerms), rbac.PermissionNames(newPerms))
				if err := tx.Save(audit).Error; err != nil {
					return fmt.Errorf("failed to save audit: %w", err)
				}
			}

			// Cascade updated_at on user
			if err := tx.
				Model(&User{}).
				Where("id = ?", membership.UserID).
				UpdateColumn("updated_at", time.Now().UTC()).
				Error; err != nil {
				return fmt.Errorf("failed to update user updated_at: %w", err)
			}
		}

		return nil
	})
}
