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
	"context"
	"fmt"
	"strings"
	"time"

	"firebase.google.com/go/auth"
	"github.com/jinzhu/gorm"
	"github.com/sethvargo/go-password/password"
)

// User represents a user of the system
type User struct {
	gorm.Model
	Errorable

	Email           string `gorm:"type:varchar(250);unique_index"`
	Name            string `gorm:"type:varchar(100)"`
	Admin           bool   `gorm:"default:false"`
	LastRevokeCheck time.Time
	Realms          []*Realm `gorm:"many2many:user_realms"`
	AdminRealms     []*Realm `gorm:"many2many:admin_realms"`

	LastPasswordChange time.Time
}

// PasswordAgeString displays the age of the password in friendly text.
func (u *User) PasswordAgeString() string {
	ago := time.Since(u.LastPasswordChange)
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
	return fmt.Sprintf("%d minutes", int(ago.Seconds()))
}

// BeforeSave runs validations. If there are errors, the save fails.
func (u *User) BeforeSave(tx *gorm.DB) error {
	u.Email = strings.TrimSpace(u.Email)
	u.Name = strings.TrimSpace(u.Name)

	if u.Email == "" {
		u.AddError("email", "cannot be blank")
	}

	if !strings.Contains(u.Email, "@") {
		u.AddError("email", "appears to be invalid")
	}

	if u.Name == "" {
		u.AddError("name", "cannot be blank")
	}

	if len(u.Errors()) > 0 {
		return fmt.Errorf("validation failed")
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
		Where("email = ?", email).
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

	start = start.Truncate(24 * time.Hour)
	stop = stop.Truncate(24 * time.Hour)

	if err := db.db.
		Model(&UserStats{}).
		Where("user_id = ?", u.ID).
		Where("realm_id = ?", realmID).
		Where("date >= ? AND date <= ?", start, stop).
		Order("date DESC").
		Find(&stats).
		Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}

	return stats, nil
}

// ListSystemAdmins returns a list of users who are system admins sorted by
// name.
func (db *Database) ListSystemAdmins() ([]*User, error) {
	var users []*User
	if err := db.db.
		Model(&User{}).
		Where("admin IS TRUE").
		Order("name DESC").
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

// CreateFirebaseUser creates the associated Firebase user for this database
// user. It does nothing if the firebase user already exists. If the firebase
// user does not exist, it generates a random password. The returned boolean
// indicates if the user was created.
func (u *User) CreateFirebaseUser(ctx context.Context, fbAuth *auth.Client) (bool, error) {
	if _, err := fbAuth.GetUserByEmail(ctx, u.Email); err != nil {
		if auth.IsInvalidEmail(err) {
			return false, fmt.Errorf("invalid email: %q", u.Email)
		}
		if !auth.IsUserNotFound(err) {
			return false, fmt.Errorf("failed lookup firebase user: %w", err)
		}

		pwd, err := password.Generate(24, 8, 8, false, true)
		if err != nil {
			return false, fmt.Errorf("failed to generate password: %w", err)
		}

		fbUser := &auth.UserToCreate{}
		fbUser = fbUser.Email(u.Email)
		fbUser = fbUser.Password(pwd)
		fbUser = fbUser.DisplayName(u.Name)
		if _, err := fbAuth.CreateUser(ctx, fbUser); err != nil {
			return false, fmt.Errorf("failed to create firebase user: %w", err)
		}
		return true, nil
	}

	return false, nil
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

// SaveUser updates the user in the database.
func (db *Database) SaveUser(u *User) error {
	db.db.Model(u).Association("Realms").Replace(u.Realms)
	db.db.Model(u).Association("AdminRealms").Replace(u.AdminRealms)
	return db.db.Save(u).Error
}
