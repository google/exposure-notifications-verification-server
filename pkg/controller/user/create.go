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

package user

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		Email string `form:"email"`
		Name  string `form:"name"`
		Admin bool   `form:"admin"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderNew(ctx, w)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w)
			return
		}

		// See if the user already exists by email - they may be a member of another
		// realm.
		user, err := c.db.FindUserByEmail(form.Email)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}

			user = new(database.User)
			user.Email = form.Email
			user.Name = form.Name
		}

		// Build the user struct
		user.Realms = append(user.Realms, realm)
		if form.Admin {
			user.AdminRealms = append(user.AdminRealms, realm)
		}

		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to create user: %v", err)
			c.renderNew(ctx, w)
			return
		}

		inviteComposer, err := controller.SendInviteEmailFunc(ctx, c.db, c.h, user.Email)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if _, err := c.authProvider.CreateUser(ctx, user.Name, user.Email, "", true, inviteComposer); err != nil {
			flash.Alert("Failed to create user: %v", err)
			c.renderNew(ctx, w)
			return
		}

		flash.Alert("Successfully created user %v.", user.Name)

		stats, err := c.getStats(ctx, user, realm)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, user, stats)
	})
}

func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter) {
	c.renderUpdate(ctx, w, &database.User{})
}
