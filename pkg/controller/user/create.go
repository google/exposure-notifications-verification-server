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

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-password/password"
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

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var user database.User
			c.renderNew(ctx, w, &user, false)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			user := &database.User{
				Email: form.Email,
				Name:  form.Name,
				Admin: form.Admin,
			}

			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w, user, false)
			return
		}

		// See if the user already exists by email - they may be a member of another
		// realm.
		user, err := c.db.FindUserByEmail(form.Email)
		alreadyExists := true
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}

			user = new(database.User)
			alreadyExists = false
		}

		// Build the user struct - keeping email and name if user already exists in another realm.
		if !alreadyExists {
			user.Email = form.Email
			user.Name = form.Name
		}
		user.Realms = append(user.Realms, realm)

		if form.Admin {
			user.AdminRealms = append(user.AdminRealms, realm)
		}

		if err := c.db.SaveUser(user); err != nil {
			flash.Error("Failed to create user: %v", err)
			c.renderNew(ctx, w, user, false)
			return
		}

		if _, err := c.client.GetUserByEmail(ctx, user.Email); auth.IsUserNotFound(err) {
			pwd, err := password.Generate(24, 8, 8, false, true)
			if err != nil {
				flash.Alert("Failed to generate password for '%v'", form.Email)
				c.renderNew(ctx, w, user, false)
				return
			}

			fbUser := &auth.UserToCreate{}
			fbUser.Email(user.Email).DisplayName(user.Name).Password(pwd)
			if _, err = c.client.CreateUser(ctx, fbUser); err != nil {
				flash.Alert("Error creating user '%v'", form.Email)
				c.renderNew(ctx, w, user, false)
				return
			}

			flash.Alert("Successfully created user '%v'", form.Name)
			c.renderNew(ctx, w, user, true)
			return
		}

		c.renderNew(ctx, w, user, false)
	})
}

func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, user *database.User, createdNewUser bool) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = user
	if createdNewUser {
		m["firebase"] = c.config.Firebase
	}
	m["created"] = createdNewUser
	c.h.RenderHTML(w, "users/new", m)
}
