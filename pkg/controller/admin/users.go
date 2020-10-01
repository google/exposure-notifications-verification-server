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

package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

// HandleUsersIndex renders the list of system admins.
func (c *Controller) HandleUsersIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		admins, err := c.db.ListSystemAdmins()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m["admins"] = admins
		c.h.RenderHTML(w, "admin/users/index", m)
	})
}

// HandleUsersCreate creates a new system admin.
func (c *Controller) HandleUsersCreate() http.Handler {
	type FormData struct {
		Email string `form:"email"`
		Name  string `form:"name"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var user database.User
			c.renderNewUser(ctx, w, &user)
			return
		}

		var form FormData
		err := controller.BindForm(w, r, &form)
		email := strings.TrimSpace(form.Email)
		name := strings.TrimSpace(form.Name)
		if err != nil {
			user := &database.User{
				Email: email,
				Name:  name,
			}

			flash.Error("Failed to process form: %v", err)
			c.renderNewUser(ctx, w, user)
			return
		}

		// See if the user already exists and use that record.
		user, err := c.db.FindUserByEmail(email)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}

			// User does not exist, create a new one.
			user = &database.User{
				Name:  name,
				Email: email,
			}
		}

		user.Admin = true
		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to create user: %v", err)
			c.renderNewUser(ctx, w, user)
			return
		}

		created, err := user.CreateFirebaseUser(ctx, c.firebaseAuth)
		if err != nil {
			flash.Alert("Failed to create user: %v", err)
			c.renderNewUser(ctx, w, user)
		}

		// Once the Go SDK supports server-side password reset emails, we can drop
		// this.
		if created {
			flash.Alert(`Successfully created user %v with a random password. `+
				`Have them set a password using the "Forgot Password" function.`, user.Email)
		}

		flash.Alert("Successfully created system admin '%v'", user.Name)
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
	})
}

func (c *Controller) renderNewUser(ctx context.Context, w http.ResponseWriter, user *database.User) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = user
	c.h.RenderHTML(w, "admin/users/new", m)
}

// HandleUsersDelete deletes a system admin.
func (c *Controller) HandleUsersDelete() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		user, err := c.db.FindUser(vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		if user.ID == currentUser.ID {
			flash.Error("Cannot remove yourself!")
			controller.Back(w, r, c.h)
			return
		}

		user.Admin = false
		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to remove system admin: %v", err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Successfully removed %v as a system admin", user.Email)
		controller.Back(w, r, c.h)
	})
}
