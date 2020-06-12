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

package datastore

import (
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"cloud.google.com/go/datastore"
)

var _ database.User = (*User)(nil)

// Used for marhsalling/unmarshalling to/from datastore.
// non-exported struct w/ exported fields so that datastore reflection library works.
type internalUser struct {
	Admin      bool           `datastore:"admin"`
	Disabled   bool           `datastore:"disabled"`
	LastRevoke time.Time      `datastore:"revoke_check,noindex"`
	K          *datastore.Key `datastore:"__key__"`
}

func (iu *internalUser) toUser() *User {
	return &User{
		admin:           iu.Admin,
		disabled:        iu.Disabled,
		lastRevokeCheck: iu.LastRevoke,
		key:             iu.K,
	}
}

type User struct {
	admin           bool
	disabled        bool
	lastRevokeCheck time.Time
	key             *datastore.Key
}

func (u *User) toInternalUser() *internalUser {
	return &internalUser{
		Admin:      u.admin,
		Disabled:   u.disabled,
		LastRevoke: u.lastRevokeCheck,
		K:          u.key,
	}
}

func (u *User) Email() string {
	return u.key.Name
}

func (u *User) Admin() bool {
	return u.admin
}

func (u *User) Disabled() bool {
	return u.disabled
}

func (u *User) LastRevokeCheck() time.Time {
	return u.lastRevokeCheck
}
