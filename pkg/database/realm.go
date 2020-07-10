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

import "github.com/jinzhu/gorm"

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	gorm.Model
	Name                   string `gorm:"type:varchar(100);unique_index"`
	KeyIDPrefix            string `gorm:"type:varchar(10);unique_index"`
	PublishVerificationKey bool   `gorm:"default:true"`

	AuthorizedApps []*AuthorizedApp

	RealmUsers  []*User `gorm:"many2many:user_realms"`
	RealmAdmins []*User

	// Relations to items that blong to a realm.
	Codes  []*VerificationCode
	Tokens []*Token
}

func (r *Realm) AddAuthorizedApp(a *AuthorizedApp) {
	r.AuthorizedApps = append(r.AuthorizedApps, a)
}

func (r *Realm) AddUser(u *User) {
	r.RealmUsers = append(r.RealmUsers, u)
}

func (r *Realm) AddAdminUser(u *User) {
	r.RealmAdmins = append(r.RealmAdmins, u)
}
