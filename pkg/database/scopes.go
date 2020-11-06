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
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/jinzhu/gorm"
)

// Scope is a type alias to a gorm scope. It exists to reduce duplicate and
// function length. Note this is an ALIAS. It is NOT a new type.
type Scope = func(db *gorm.DB) *gorm.DB

// OnlySystemAdmins returns a scope that restricts the query to system admins.
// It's only applicable to functions that query User.
func OnlySystemAdmins() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(&User{SystemAdmin: true})
	}
}

// OnlyRealmAdmins returns a scope that restricts the query to users that are
// administrators of 1 or more realms. It's only applicable to functions that
// query User.
func OnlyRealmAdmins() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Joins("INNER JOIN (SELECT DISTINCT user_id FROM admin_realms) ar ON users.id = ar.user_id")
	}
}

// WithUserSearch returns a scope that adds querying for users by email and
// name, case-insensitive. It's only applicable to functions that query User.
func WithUserSearch(q string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		q = project.TrimSpace(q)
		if q != "" {
			q = `%` + q + `%`
			return db.Where("users.email ILIKE ? OR users.name ILIKE ?", q, q)
		}
		return db
	}
}
