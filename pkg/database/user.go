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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/opencensus-integrations/redigo/redis"
)

// User represents a user of the system
type User struct {
	gorm.Model
	Email           string `gorm:"type:varchar(250);unique_index"`
	Name            string `gorm:"type:varchar(100)"`
	Admin           bool   `gorm:"default:false"`
	Disabled        bool
	LastRevokeCheck time.Time
	Realms          []*Realm `gorm:"many2many:user_realms;PRELOAD:true"`
	AdminRealms     []*Realm `gorm:"many2many:admin_realms;PRELOAD:true"`
}

func (u *User) MultipleRealms() bool {
	return len(u.Realms) > 1
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

// ListUsers retrieves all of the configured users.
// Done without pagination.
// This is not scoped to realms.
func (db *Database) ListUsers(includeDeleted bool) ([]*User, error) {
	var users []*User

	scope := db.db
	if includeDeleted {
		scope = db.db.Unscoped()
	}
	if err := scope.Order("email ASC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	return users, nil
}

// GetUser reads back a User struct by email address.
func (db *Database) GetUser(id uint) (*User, error) {
	if cachedUser, err := db.fetchCachedUserByID(context.Background(), id); err == nil && cachedUser != nil {
		return cachedUser, nil
	}

	var user User
	if err := db.db.Preload("Realms").Preload("AdminRealms").Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUser reads back a User struct by email address.
func (db *Database) FindUser(email string) (*User, error) {
	if cachedUser, err := db.fetchCachedUserByEmail(context.Background(), email); err == nil && cachedUser != nil {
		return cachedUser, nil
	}

	var user User
	if err := db.db.Preload("Realms").Preload("AdminRealms").Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser creates a user record.
func (db *Database) CreateUser(email string, name string, admin bool, disabled bool) (*User, error) {
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("provided email address may not be valid, double check: '%v'", email)
	}

	if name == "" {
		name = parts[0]
	}

	user, err := db.FindUser(email)
	if err == gorm.ErrRecordNotFound {
		// New record.
		user = &User{}
	} else if err != nil {
		return nil, err
	}

	// Update fields
	user.Email = email
	user.Name = name
	user.Admin = admin
	user.Disabled = disabled

	if err := db.SaveUser(user); err != nil {
		return nil, err
	}

	// After saving the user, now cache it.

	return user, nil
}

// SaveUser creates or updates a user record.
func (db *Database) SaveUser(u *User) (err error) {
	defer func() {
		if err == nil {
			db.cacheUser(context.Background(), u)
		}
	}()

	if u.Model.ID == 0 {
		return db.db.Create(u).Error
	}
	return db.db.Save(u).Error
}

// DeleteUser removes a user record.
func (db *Database) DeleteUser(u *User) error {
	// Ensure we remove any vestiges of that User from the cache.
	defer db.purgeUsersFromCache(context.Background(), u)

	return db.db.Delete(u).Error
}

// PurgeUsers will remove users records that are disabled and haven't been updated
// within the provided duration.
// This is a hard delete, not a soft delete.
func (db *Database) PurgeUsers(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	var recvUsers []*User
	scope := db.db.Unscoped()
	whereClause := "disabled = ? and updated_at < ?"
	if err := scope.Where(whereClause, true, deleteBefore).Find(&recvUsers).Error; err == nil {
		// Also clear our Redis caches of the respective records.
		defer db.purgeUsersFromCache(context.Background(), recvUsers...)
	}
	rtn := scope.Where(whereClause, true, deleteBefore).Delete(&User{})
	return rtn.RowsAffected, rtn.Error
}

// This TTL is set to 1 day so that in the worst case, if a deletion happens but
// doesn't clear the cache, it'll eventually expire, which ensures eventual "deletion".
const _CACHE_PURGE_TTL_1DAY = 24 * time.Hour

func (db *Database) cacheUser(ctx context.Context, user *User) error {
	conn, err := db.getRedisConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	jsonBlob, err := json.Marshal(user)
	if err != nil {
		return err
	}

	conn.SendContext(ctx, "MULTI")
	conn.SendContext(ctx, "SET", _USERS_TABLE_BY_EMAIL, user.Email, jsonBlob)
	conn.SendContext(ctx, "EXPIRE", _USERS_TABLE_BY_EMAIL, user.Email, _CACHE_PURGE_TTL_1DAY.Seconds())
	conn.SendContext(ctx, "SET", _USERS_TABLE_BY_ID, user.ID, jsonBlob)
	conn.SendContext(ctx, "EXPIRE", _USERS_TABLE_BY_ID, user.ID, _CACHE_PURGE_TTL_1DAY.Seconds())
	// Use a Time to live (TTL) of 1days so that for example if succeeded but clearing it from the
	// Redis cache failed, the cached information will at least be purged if not used within within 1day.
	_, err = conn.DoContext(ctx, "EXEC")
	return err
}

const (
	_USERS_TABLE_BY_EMAIL = "USERS_email"
	_USERS_TABLE_BY_ID    = "USERS_id"
)

func (db *Database) purgeUsersFromCache(ctx context.Context, users ...*User) error {
	if len(users) == 0 {
		return nil
	}

	conn, err := db.getRedisConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SendContext(ctx, "MULTI")
	for _, user := range users {
		conn.SendContext(ctx, "DEL", _USERS_TABLE_BY_EMAIL, user.Email)
		conn.SendContext(ctx, "DEL", _USERS_TABLE_BY_ID, user.ID)
	}
	_, err = conn.DoContext(ctx, "EXEC")
	return err
}

func (db *Database) fetchCachedUserByID(ctx context.Context, id uint) (*User, error) {
	return db.fetchCachedUser(ctx, _USERS_TABLE_BY_ID, id)
}

func (db *Database) fetchCachedUserByEmail(ctx context.Context, email string) (*User, error) {
	return db.fetchCachedUser(ctx, _USERS_TABLE_BY_EMAIL, email)
}

func (db *Database) fetchCachedUser(ctx context.Context, tableName string, key interface{}) (*User, error) {
	conn, err := db.getRedisConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	jsonBlob, err := redis.Bytes(conn.DoContext(ctx, "GET", tableName, key))
	if err != nil {
		return nil, err
	}
	if len(jsonBlob) == 0 {
		return nil, errNoUser
	}
	recv := new(User)
	if err := json.Unmarshal(jsonBlob, recv); err != nil {
		return nil, err
	}
	return recv, nil
}

var (
	errRedisDisabled = errors.New("database: redis is disabled")
	errNoUser        = errors.New("database: no user found in redis cache")
)

func (db *Database) getRedisConn(ctx context.Context) (redis.ConnWithContext, error) {
	db.redisMu.RLock()
	defer db.redisMu.RUnlock()

	redisPool := db.redisPool
	if redisPool == nil {
		return nil, errRedisDisabled
	}

	conn, err := redisPool.Dial()
	if err != nil {
		return nil, err
	}
	return conn.(redis.ConnWithContext), nil
}
