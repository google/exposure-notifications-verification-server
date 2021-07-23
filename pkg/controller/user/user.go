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

// Package user contains web controllers for listing and adding users.
package user

import (
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	authProvider auth.Provider
	cacher       cache.Cacher
	db           *database.Database
	h            *render.Renderer
}

func New(authProvider auth.Provider, cacher cache.Cacher, db *database.Database, h *render.Renderer) *Controller {
	return &Controller{
		cacher:       cacher,
		authProvider: authProvider,
		db:           db,
		h:            h,
	}
}

func (c *Controller) findUser(currentUser *database.User, realm *database.Realm, id interface{}) (*database.User, *database.Membership, error) {
	var user *database.User
	var err error

	// Look up the user.
	if currentUser.SystemAdmin {
		user, err = c.db.FindUser(id)
	} else {
		user, err = realm.FindUser(c.db, id)
	}
	if err != nil {
		return nil, nil, err
	}

	membership, err := user.FindMembership(c.db, realm.ID)
	if err != nil {
		return nil, nil, err
	}

	return user, membership, nil
}
