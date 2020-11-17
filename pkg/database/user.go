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
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

const minDuration = -1 << 63

// They probably didn't make an account before this project existed.
var launched time.Time = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)

// Ensure user can be an audit actor.
var _ Auditable = (*User)(nil)

// User represents a user of the system
type User struct {
	gorm.Model
	Errorable

	Email       string `gorm:"type:varchar(250);unique_index"`
	Name        string `gorm:"type:varchar(100)"`
	SystemAdmin bool   `gorm:"column:system_admin; default:false;"`

	Realms      []*Realm `gorm:"many2many:user_realms"`
	AdminRealms []*Realm `gorm:"many2many:admin_realms"`

	LastRevokeCheck    time.Time
	LastPasswordChange time.Time
}

// PasswordChanged returns password change time or account creation time if unset.
func (u *User) PasswordChanged() time.Time {
	if u.LastPasswordChange.Before(launched) {
		return u.CreatedAt
	}
	return u.LastPasswordChange
}

// AfterFind runs after the record is found.
func (u *User) AfterFind(tx *gorm.DB) error {
	// Sort Realms and Admin realms. Unfortunately gorm provides no way to do this
	// via sql hooks or default scopes.
	sort.Slice(u.Realms, func(i, j int) bool {
		return strings.ToLower(u.Realms[i].Name) < strings.ToLower(u.Realms[j].Name)
	})
	sort.Slice(u.AdminRealms, func(i, j int) bool {
		return strings.ToLower(u.AdminRealms[i].Name) < strings.ToLower(u.AdminRealms[j].Name)
	})

	return nil
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

// BeforeSave runs validations. If there are errors, the save fails.
func (u *User) BeforeSave(tx *gorm.DB) error {
	// Validation
	u.Email = project.TrimSpace(u.Email)
	if u.Email == "" {
		u.AddError("email", "cannot be blank")
	}
	if !strings.Contains(u.Email, "@") {
		u.AddError("email", "appears to be invalid")
	}

	u.Name = project.TrimSpace(u.Name)
	if u.Name == "" {
		u.AddError("name", "cannot be blank")
	}

	if len(u.Errors()) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(u.ErrorMessages(), ", "))
	}

	return nil
}

func (u *User) GetRealm(realmID uint) *Realm {
	for _, r := range u.Realms {
		if r.ID == realmID {
			return r
		}
	}
	return nil
}

func (u *User) CanViewRealm(realmID uint) bool {
	for _, r := range u.Realms {
		if r.ID == realmID {
			return true
		}
	}
	return false
}

func (u *User) IsRealmAdmin() bool {
	return len(u.AdminRealms) > 0
}

func (u *User) CanAdminRealm(realmID uint) bool {
	for _, r := range u.AdminRealms {
		if r.ID == realmID {
			return true
		}
	}
	return false
}

// AddRealm adds the user to the realm.
func (u *User) AddRealm(realm *Realm) {
	u.Realms = append(u.Realms, realm)
}

// AddRealmAdmin adds the user to the realm as an admin.
func (u *User) AddRealmAdmin(realm *Realm) {
	u.AdminRealms = append(u.AdminRealms, realm)
	u.AddRealm(realm)
}

// RemoveRealm removes the user from the realm. It also removes the user as an
// admin of that realm. You must save the user to persist the changes.
func (u *User) RemoveRealm(realm *Realm) {
	for i, r := range u.Realms {
		if r.ID == realm.ID {
			u.Realms = append(u.Realms[:i], u.Realms[i+1:]...)
		}
	}
	u.RemoveRealmAdmin(realm)
}

// RemoveRealmAdmin removes the user from the realm. You must save the user to
// persist the changes.
func (u *User) RemoveRealmAdmin(realm *Realm) {
	for i, r := range u.AdminRealms {
		if r.ID == realm.ID {
			u.AdminRealms = append(u.AdminRealms[:i], u.AdminRealms[i+1:]...)
		}
	}
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

// Stats returns the usage statistics for this user at the provided realm. If no
// stats exist, it returns an empty array.
func (u *User) Stats(db *Database, realmID uint, start, stop time.Time) ([]*UserStats, error) {
	var stats []*UserStats

	start = timeutils.Midnight(start)
	stop = timeutils.Midnight(stop)

	if err := db.db.
		Model(&UserStats{}).
		Where("user_id = ?", u.ID).
		Where("realm_id = ?", realmID).
		Where("date >= ? AND date <= ?", start, stop).
		Order("date ASC").
		Find(&stats).
		Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
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

// ListSystemAdmins returns a list of users who are system admins sorted by
// name.
func (db *Database) ListSystemAdmins() ([]*User, error) {
	var users []*User
	if err := db.db.
		Model(&User{}).
		Where("system_admin IS TRUE").
		Order("LOWER(name) ASC").
		Find(&users).
		Error; err != nil {
		if IsNotFound(err) {
			return users, nil
		}
		return nil, err
	}

	return users, nil
}

// TouchUserRevokeCheck updates the revoke check time on the user. It updates
// the column directly and does not invoke callbacks.
func (db *Database) TouchUserRevokeCheck(u *User) error {
	return db.db.
		Model(u).
		UpdateColumn("last_revoke_check", time.Now().UTC()).
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
		if err := tx.Delete(u).Error; err != nil {
			return fmt.Errorf("failed to save user: %w", err)
		}

		return nil
	})
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

		// Force-update associations
		tx.Model(u).Association("Realms").Replace(u.Realms)
		tx.Model(u).Association("AdminRealms").Replace(u.AdminRealms)

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

		// Diff realms - this intentionally happens for both new and existing users
		existingRealms := make(map[uint]struct{}, len(existing.Realms))
		for _, v := range existing.Realms {
			existingRealms[v.ID] = struct{}{}
		}
		existingAdminRealms := make(map[uint]struct{}, len(existing.AdminRealms))
		for _, v := range existing.AdminRealms {
			existingAdminRealms[v.ID] = struct{}{}
		}

		newRealms := make(map[uint]struct{}, len(u.Realms))
		for _, v := range u.Realms {
			newRealms[v.ID] = struct{}{}
		}
		newAdminRealms := make(map[uint]struct{}, len(u.AdminRealms))
		for _, v := range u.AdminRealms {
			newAdminRealms[v.ID] = struct{}{}
		}

		for ear := range existingAdminRealms {
			if _, ok := newAdminRealms[ear]; !ok {
				audit := BuildAuditEntry(actor, "demoted user from realm admin", u, ear)
				audits = append(audits, audit)
			}
		}

		for er := range existingRealms {
			if _, ok := newRealms[er]; !ok {
				audit := BuildAuditEntry(actor, "removed user from realm", u, er)
				audits = append(audits, audit)
			}
		}

		for nr := range newRealms {
			if _, ok := existingRealms[nr]; !ok {
				audit := BuildAuditEntry(actor, "added user to realm", u, nr)
				audits = append(audits, audit)
			}
		}

		for nr := range newAdminRealms {
			if _, ok := existingAdminRealms[nr]; !ok {
				audit := BuildAuditEntry(actor, "promoted user to realm admin", u, nr)
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
