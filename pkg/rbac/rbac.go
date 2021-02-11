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

// Package rbac implements authorization.
package rbac

import (
	"database/sql/driver"
	"fmt"
	"sort"
)

var (
	// PermissionMap is the list of permissions mapped to their name and
	// description.
	PermissionMap = map[Permission][2]string{
		AuditRead:      {"AuditRead", "read event and audit logs"},
		APIKeyRead:     {"APIKeyRead", "view information about API keys, including statistics"},
		APIKeyWrite:    {"APIKeyWrite", "create, update, and delete API keys"},
		CodeIssue:      {"CodeIssue", "issue codes"},
		CodeBulkIssue:  {"CodeBulkIssue", "issue codes in bulk, if bulk issue is enabled on the realm"},
		CodeRead:       {"CodeRead", "lookup code status"},
		CodeExpire:     {"CodeExpire", "expire codes"},
		SettingsRead:   {"SettingsRead", "read realm settings"},
		SettingsWrite:  {"SettingsWrite", "update realm settings"},
		StatsRead:      {"StatsRead", "view realm statistics"},
		MobileAppRead:  {"MobileAppRead", "view mobile app information"},
		MobileAppWrite: {"MobileAppWrite", "create, update, and delete mobile apps"},
		UserRead:       {"UserRead", "view user information"},
		UserWrite:      {"UserWrite", "create, update, and delete users"},
	}

	// NamePermissionMap is the map of permission names to their value.
	NamePermissionMap map[string]Permission
)

func init() {
	NamePermissionMap = make(map[string]Permission, len(PermissionMap))
	for k, v := range PermissionMap {
		NamePermissionMap[v[0]] = k
	}
}

// Can returns true if the given resource has permission to perform the provided
// permissions.
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

	// Ensure implied permissions. The actor must also have the implied
	// permissions by definition.
	permission = AddImplied(permission)
	return permission, nil
}

// AddImplied adds any missing implied permissions.
func AddImplied(target Permission) Permission {
	for has, needs := range requiredPermission {
		// If granted has, ensure that we have all needs.
		if Can(target, has) {
			for _, required := range needs {
				target = target | required
			}
		}
	}
	return target
}

// ImpliedBy returns any permissions that cause this permission to be added
// automatically. The return may be nil.
func ImpliedBy(permission Permission) []Permission {
	return impliedBy[permission]
}

// PermissionNames returns the list of permissions included in the given
// permission.
func PermissionNames(p Permission) []string {
	names := make([]string, 0, len(PermissionMap))
	for v, k := range PermissionMap {
		if Can(p, v) {
			names = append(names, k[0])
		}
	}
	sort.Strings(names)
	return names
}

// Permission is a granular permission. It is an integer instead of a uint
// because most database systems lack unsigned integer types.
type Permission int64

// String implements stringer.
func (p Permission) String() string {
	if v, ok := PermissionMap[p]; ok {
		return v[0]
	}
	return fmt.Sprintf("Permission(%d)", int64(p))
}

// Value returns the permissions value as an integer for sql drivers.
func (p Permission) Value() (driver.Value, error) {
	return int64(p), nil
}

// Description returns the description.
func (p Permission) Description() (string, error) {
	if v, ok := PermissionMap[p]; ok {
		return v[1], nil
	}
	return "", fmt.Errorf("missing description for %s", p)
}

// Implied returns the additional implied permissions, if any.
func (p Permission) Implied() []Permission {
	return requiredPermission[p]
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
// Required / Implied permissions.
// Write permissions require subordinate read.
// --

var (
	// requiredPermissions is not exported since maps cannot be constant.
	requiredPermission = map[Permission][]Permission{
		APIKeyWrite:    {APIKeyRead},
		CodeBulkIssue:  {CodeIssue},
		SettingsWrite:  {SettingsRead},
		MobileAppWrite: {MobileAppRead},
		UserWrite:      {UserRead},
	}

	// This is the inverse of the above map, set by the init() func.
	// Done in code to ensure it always stays in sync with requiredPermission.
	impliedBy = make(map[Permission][]Permission)
)

// Note: there are multiple init functions in this file. They are organized to be
// near the thing they are initializing.
// Yes, go allows multiple init functions in the same module.
func init() {
	for has, needs := range requiredPermission {
		for _, perm := range needs {
			if _, ok := impliedBy[perm]; !ok {
				impliedBy[perm] = make([]Permission, 0, 1)
			}
			impliedBy[perm] = append(impliedBy[perm], has)
		}
	}
}

// --
// Legacy permissions
// --

const (
	// LegacyRealmUser is a quick reference to the old "user" permissions.
	LegacyRealmUser Permission = CodeIssue | CodeBulkIssue | CodeRead | CodeExpire

	// LegacyRealmAdmin is a quick reference to the old "realm admin" permissions.
	LegacyRealmAdmin Permission = AuditRead |
		APIKeyRead | APIKeyWrite |
		CodeIssue | CodeBulkIssue | CodeRead | CodeExpire |
		SettingsRead | SettingsWrite |
		StatsRead |
		MobileAppRead | MobileAppWrite |
		UserRead | UserWrite
)
