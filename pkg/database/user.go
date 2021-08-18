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
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
)

const minDuration = -1 << 63

// They probably didn't make an account before this project existed.
var launched = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)

// Ensure user can be an audit actor.
var _ Auditable = (*User)(nil)

// User represents a user of the system
type User struct {
	gorm.Model
	Errorable

	Email       string `gorm:"type:varchar(250);unique_index"`
	Name        string `gorm:"type:varchar(100)"`
	SystemAdmin bool   `gorm:"column:system_admin; default:false;"`

	LastRevokeCheck    time.Time
	LastPasswordChange time.Time
}

// BeforeSave runs validations. If there are errors, the save fails.
func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Email = project.TrimSpace(u.Email)
	if u.Email == "" {
		u.AddError("email", "cannot be blank")
	} else if !strings.Contains(u.Email, "@") {
		u.AddError("email", "invalid email address")
	}

	u.Name = project.TrimSpace(u.Name)
	if u.Name == "" {
		u.AddError("name", "cannot be blank")
	}

	return u.ErrorOrNil()
}

// PasswordChanged returns password change time or account creation time if unset.
func (u *User) PasswordChanged() time.Time {
	if u.LastPasswordChange.Before(launched) {
		return u.CreatedAt
	}
	return u.LastPasswordChange
}

// PasswordAgeString displays the age of the password in friendly text.
func (u *User) PasswordAgeString() string {
	ago := time.Since(u.PasswordChanged())
	if ago == minDuration {
		return "unknown"
	}

	h := ago.Hours()
	if h > 48 {
		return fmt.Sprintf("%v days", int(h/24))
	}
	if h > 2 {
		return fmt.Sprintf("%d hours", int(h))
	}
	if ago.Minutes() > 2 {
		return fmt.Sprintf("%d minutes", int(ago.Minutes()))
	}
	return fmt.Sprintf("%d seconds", int(ago.Seconds()))
}

// FindUser finds a user by the given id, if one exists. The id can be a string
// or integer value. It returns an error if the record is not found.
func (db *Database) FindUser(id interface{}) (*User, error) {
	var user User
	if err := db.db.
		Where("id = ?", id).
		First(&user).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByEmail reads back a User struct by email address. It returns an
// error if the record is not found.
func (db *Database) FindUserByEmail(email string) (*User, error) {
	var user User
	if err := db.db.
		Where("email = ?", project.TrimSpace(email)).
		First(&user).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
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

// SelectFirstMembership selects the first memberships for this user.
func (u *User) SelectFirstMembership(db *Database) (*Membership, error) {
	var membership Membership
	if err := db.db.
		Preload("Realm").
		Preload("User").
		Model(&Membership{}).
		Where("user_id = ?", u.ID).
		First(&membership).
		Error; err != nil {
		return nil, err
	}
	return &membership, nil
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

		conflict := `ON CONFLICT (user_id, realm_id) DO UPDATE SET
			permissions = EXCLUDED.permissions,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at`
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
		if then, now := existing.Permissions, permissions; then != now {
			audit := BuildAuditEntry(actor, "updated user permissions", u, r.ID)
			audit.Diff = stringSliceDiff(rbac.PermissionNames(then), rbac.PermissionNames(now))
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

// Stats returns the usage statistics for this user at the provided realm. If no
// stats exist, it returns an empty array.
func (u *User) Stats(db *Database, realm *Realm) (UserStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	// Pull the stats by generating the full date range, then join on stats. This
	// will ensure we have a full list (with values of 0 where appropriate) to
	// ensure continuity in graphs.
	sql := `
		SELECT
			d.date AS date,
			$1 AS realm_id,
			$2 AS user_id,
			$3 AS user_name,
			$4 AS user_email,
			COALESCE(s.codes_issued, 0) AS codes_issued
		FROM (
			SELECT date::date FROM generate_series($5, $6, '1 day'::interval) date
		) d
		LEFT JOIN user_stats s ON s.user_id = $2 AND s.realm_id = $1 AND s.date = d.date
		ORDER BY date DESC`

	var stats []*UserStat
	if err := db.db.Raw(sql, realm.ID, u.ID, u.Name, u.Email, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// StatsCached is stats, but cached.
func (u *User) StatsCached(ctx context.Context, db *Database, cacher cache.Cacher, realm *Realm) (UserStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats UserStats
	cacheKey := &cache.Key{
		Namespace: "stats:user",
		Key:       fmt.Sprintf("%d/%d", realm.ID, u.ID),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return u.Stats(db, realm)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// ListUsers returns a list of all users sorted by name.
// Warning: This list may be large. Use Realm.ListUsers() to get users scoped to a realm.
func (db *Database) ListUsers(p *pagination.PageParams, scopes ...Scope) ([]*User, *pagination.Paginator, error) {
	var users []*User
	query := db.db.Model(&User{}).
		Scopes(scopes...).
		Order("LOWER(name) ASC")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &users, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return users, nil, nil
		}
		return nil, nil, err
	}

	return users, paginator, nil
}

// TouchUserRevokeCheck updates the revoke check time on the user. It updates
// the column directly and does not invoke callbacks.
func (db *Database) TouchUserRevokeCheck(u *User) error {
	return db.db.
		Model(u).
		UpdateColumn("last_revoke_check", time.Now().UTC()).
		Error
}

// UntouchUserRevokeCheck removes the last revoke check, forcing it to occur on
// next auth.
func (db *Database) UntouchUserRevokeCheck(u *User) error {
	return db.db.
		Model(u).
		UpdateColumn("last_revoke_check", nil).
		Error
}

// PasswordChanged updates the last password change timestamp of the user.
func (db *Database) PasswordChanged(email string, t time.Time) error {
	q := db.db.
		Model(&User{}).
		Where("email = ?", email).
		UpdateColumn("last_password_change", t.UTC())
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return fmt.Errorf("no rows affected user %s", email)
	}
	return nil
}

// AuditID is how the user is stored in the audit entry.
func (u *User) AuditID() string {
	return fmt.Sprintf("users:%d", u.ID)
}

// AuditDisplay is how the user will be displayed in audit entries.
func (u *User) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", u.Name, u.Email)
}

// DeleteUser deletes the user entry.
func (db *Database) DeleteUser(u *User, actor Auditable) error {
	if u == nil {
		return fmt.Errorf("provided user is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		audit := BuildAuditEntry(actor, "deleted user", u, 0)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		// Delete the user
		if err := tx.Unscoped().Delete(u).Error; err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		return nil
	})
}

// PurgeUsers will delete users who are not a system admin, not a member of any realms
// and have not been modified before the expiry time.
func (db *Database) PurgeUsers(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	// Delete users who were created/updated before the expiry time.
	rtn := db.db.Unscoped().
		Where("users.system_admin = false AND users.created_at < ? AND users.updated_at < ?", deleteBefore, deleteBefore).
		Where("NOT EXISTS(SELECT 1 FROM memberships WHERE memberships.user_id = users.id LIMIT 1)"). // delete where no realm association exists.
		Delete(&User{})
	return rtn.RowsAffected, rtn.Error
}

func (db *Database) SaveUser(u *User, actor Auditable) error {
	if u == nil {
		return fmt.Errorf("provided user is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		// Look up the existing user so we can do a diff and generate audit entries.
		var existing User
		if err := tx.
			Model(&User{}).
			Where("id = ?", u.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing user")
		}

		// Save the user
		if err := tx.Save(u).Error; err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		// Brand new user?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created user", u, 0)
			audits = append(audits, audit)
		} else {
			if existing.SystemAdmin != u.SystemAdmin {
				audit := BuildAuditEntry(actor, "updated user system admin", u, 0)
				audit.Diff = boolDiff(existing.SystemAdmin, u.SystemAdmin)
				audits = append(audits, audit)
			}

			if existing.Name != u.Name {
				audit := BuildAuditEntry(actor, "updated user's name", u, 0)
				audit.Diff = stringDiff(existing.Name, u.Name)
				audits = append(audits, audit)
			}

			if existing.Email != u.Email {
				audit := BuildAuditEntry(actor, "updated user's email", u, 0)
				audit.Diff = stringDiff(existing.Email, u.Email)
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
