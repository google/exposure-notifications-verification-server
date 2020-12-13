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

//go:generate stringer -output=rbac_gen.go -type=Permission

// Package rbac implements authorization.
package rbac

import (
	"database/sql/driver"
	"fmt"
	"sort"
)

// Can returns true if the given resource has permission to perform ALL of the
// provided permissions.
func Can(given Permission, target Permission) bool {
	return int64(given)&int64(target) != 0
}

// CompileAndAuthorize compiles a new permission bit from the given toUpdate
// permissions. It verifies that the calling permission has a superset of all
// provided permissions (to prevent privilege escalation).
func CompileAndAuthorize(actorPermission Permission, toUpdate []Permission) (Permission, error) {
	var permission Permission
	for _, update := range toUpdate {
		// Verify that the user making changes has the permissions they are trying
		// to grant. It is not valid for someone to grant permissions larger than
		// they currently have.
		if !Can(actorPermission, update) {
			return 0, fmt.Errorf("actor does not have all scopes which are being granted")
		}
		permission = permission | update
	}
	return permission, nil
}

// PermissionMap is a map of permissions to their names. It requires the
// stringer generation.
func PermissionMap() map[string]Permission {
	m := make(map[string]Permission, len(_Permission_map)+2)
	for k, v := range _Permission_map {
		m[v] = k
	}
	return m
}

// PermissionNames returns the list of permissions included in the given
// permission.
func PermissionNames(p Permission) []string {
	names := make([]string, 0, len(_Permission_map))
	for v, k := range _Permission_map {
		if Can(p, v) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// Permission is a granular permission. It is an integer instead of a uint
// because most database systems lack unsigned integer types.
type Permission int64

// Value returns the permissions value as an integer for sql drivers.
func (p Permission) Value() (driver.Value, error) {
	return int64(p), nil
}

const (
	_ Permission = 1 << iota

	// Audit
	AuditRead

	// API keys
	APIKeyRead
	APIKeyWrite

	// Codes
	CodeIssue
	CodeBulkIssue
	CodeRead
	CodeExpire

	// Realm settings
	SettingsRead
	SettingsWrite

	// Realm statistics
	StatsRead

	// Mobile apps
	MobileAppRead
	MobileAppWrite

	// Users
	UserRead
	UserWrite
)

// --
// Legacy permissions
// --

const (
	// LegacyRealmUser is a quick reference to the old "user" permissions.
	LegacyRealmUser = CodeIssue | CodeBulkIssue | CodeRead | CodeExpire

	// LegacyRealmAdmin is a quick reference to the old "realm admin" permissions.
	LegacyRealmAdmin = AuditRead |
		APIKeyRead | APIKeyWrite |
		CodeIssue | CodeBulkIssue | CodeRead | CodeExpire |
		SettingsRead | SettingsWrite |
		StatsRead |
		MobileAppRead | MobileAppWrite |
		UserRead | UserWrite
)
