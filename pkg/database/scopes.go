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
	"strings"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/jinzhu/gorm"
)

// Scope is a type alias to a gorm scope. It exists to reduce duplicate and
// function length. Note this is an ALIAS. It is NOT a new type.
type Scope = func(db *gorm.DB) *gorm.DB

// Unscoped returns an unscoped database (for finding soft-deleted records and
// clearing other scopes).
func Unscoped() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Unscoped()
	}
}

// OnlySystemAdmins returns a scope that restricts the query to system admins.
// It's only applicable to functions that query User.
func OnlySystemAdmins() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(&User{SystemAdmin: true})
	}
}

// InConsumableSecretOrder is a scope that orders secrets in the order in which
// they should be consumed.
func InConsumableSecretOrder() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("secrets.active DESC, secrets.created_at DESC")
	}
}

// WithUserSearch returns a scope that adds querying for users by email and
// name, case-insensitive. It's only applicable to functions that query User.
func WithUserSearch(q string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		search, err := parseUserSearch(q)
		if err != nil {
			_ = db.AddError(fmt.Errorf("%w: %s", ErrValidationFailed, err))
			return db
		}

		if search.name != "" {
			db = db.Where("users.name ~* ?", fmt.Sprintf("(%s)", search.name))
		}

		if search.email != "" {
			db = db.Where("users.email ~* ?", fmt.Sprintf("(%s)", search.email))
		}

		// For backwards-compatibility with previous versions of search, other could
		// have been a name or email.
		if search.other != nil {
			s := strings.Join(search.other, "|")
			db = db.Where("users.name ~* ? OR users.email ~* ?", fmt.Sprintf("(%s)", s), fmt.Sprintf("(%s)", s))
		}

		if p := search.withPerms; p != 0 {
			db = WithPermissionSearch(p)(db)
		}

		if p := search.withoutPerms; p != 0 {
			db = WithoutPermissionSearch(p)(db)
		}

		return db
	}
}

type userSearchQuery struct {
	name  string
	email string
	other []string

	withPerms    rbac.Permission
	withoutPerms rbac.Permission
}

func parseUserSearch(q string) (*userSearchQuery, error) {
	var resp userSearchQuery

	parts := strings.Split(project.TrimSpace(q), " ")
	for _, part := range parts {
		part = project.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "name:"):
			if resp.name != "" {
				return nil, fmt.Errorf(`cannot specify "name:" more than once`)
			}
			resp.name = project.TrimSpace(part[5:])
		case strings.HasPrefix(part, "email:"):
			if resp.email != "" {
				return nil, fmt.Errorf(`cannot specify "email:" more than once`)
			}
			resp.email = project.TrimSpace(part[6:])
		case strings.HasPrefix(part, "can:"):
			name := project.TrimSpace(part[4:])
			p, ok := rbac.NamePermissionMap[name]
			if !ok {
				return nil, fmt.Errorf("unknown permission %q", name)
			}
			resp.withPerms |= p
		case strings.HasPrefix(part, "cannot:"):
			name := project.TrimSpace(part[7:])
			p, ok := rbac.NamePermissionMap[name]
			if !ok {
				return nil, fmt.Errorf("unknown permission %q", name)
			}
			resp.withoutPerms |= p
		default:
			// These are "naked" fields in the query
			resp.other = append(resp.other, part)
		}
	}

	return &resp, nil
}

// WithPermissionSearch searches for memberships which have the given
// permission.
func WithPermissionSearch(p rbac.Permission) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(fmt.Sprintf("memberships.permissions::bit(64) & %d::bit(64) != 0::bit(64)", p))
	}
}

// WithoutPermissionSearch searches for memberships which do not have the given
// permission.
func WithoutPermissionSearch(p rbac.Permission) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(fmt.Sprintf("memberships.permissions::bit(64) & %d::bit(64) = 0::bit(64)", p))
	}
}

// WithAuditTime returns a scope that adds querying for Audit events by time.
func WithAuditTime(from, to string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		from = project.TrimSpace(from)
		if from != "" {
			db = db.Where("audit_entries.created_at >= ?", from)
		}

		to = project.TrimSpace(to)
		if to != "" {
			db = db.Where("audit_entries.created_at <= ?", to)
		}
		return db
	}
}

// WithAuditRealmID returns a scope that adds querying for Audit events by
// realm. The provided ID is expected to be stringable (int, uint, string).
func WithAuditRealmID(id uint) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("audit_entries.realm_id = ?", id)
	}
}

// WithRealmSearch returns a scope that adds querying for realms by name. It's
// only applicable to functions that query Realm.
func WithRealmSearch(q string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		q = project.TrimSpace(q)
		if q != "" {
			q = `%` + q + `%`
			return db.Where("realms.name ILIKE ?", q)
		}
		return db
	}
}

// WithRealmAutoKeyRotationEnabled filters by realms which have the auto key
// rotation enabled/disabled depending on the boolean.
func WithRealmAutoKeyRotationEnabled(b bool) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("auto_rotate_certificate_key = ?", b)
	}
}

// WithoutAuditTest excludes audit entries related to test entries created from
// SystemTest.
func WithoutAuditTest() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("audit_entries.actor_id != ?", SystemTest.AuditID())
	}
}

// WithAppOS returns a scope that for querying MobileApps by Operating System type.
func WithAppOS(os OSType) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("os = ?", os)
	}
}

// WithAuthorizedAppType returns a scope that filters by the given type.
func WithAuthorizedAppType(typ APIKeyType) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("api_key_type = ?", typ)
	}
}

// WithAuthorizedAppSearch returns a scope that adds querying for API keys by
// name and preview, case-insensitive. It's only applicable to functions that
// query AuthorizedApp.
func WithAuthorizedAppSearch(q string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		q = project.TrimSpace(q)
		if q != "" {
			q = `%` + q + `%`
			return db.Where("authorized_apps.name ILIKE ? OR authorized_apps.api_key_preview ILIKE ?", q, q)
		}
		return db
	}
}

// WithMobileAppSearch returns a scope that adds querying for mobile apps by
// name, case-insensitive. It's only applicable to functions that query
// MobileApp.
func WithMobileAppSearch(q string) Scope {
	return func(db *gorm.DB) *gorm.DB {
		q = project.TrimSpace(q)
		if q != "" {
			q = `%` + q + `%`
			return db.Where("mobile_apps.name ILIKE ? OR realms.name ILIKE ?", q, q)
		}
		return db
	}
}
