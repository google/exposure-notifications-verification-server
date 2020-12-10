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

	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// Membership represents a user's membership in a realm.
type Membership struct {
	UserID uint
	User   *User

	RealmID uint
	Realm   *Realm

	Permissions rbac.Permission
}

// AfterFind does a sanity check to ensure the User and Realm properties were
// preloaded and the referenced values exist.
func (m *Membership) AfterFind() error {
	if m.User == nil {
		return fmt.Errorf("membership user does not exist")
	}

	if m.Realm == nil {
		return fmt.Errorf("membership realm does not exist")
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
