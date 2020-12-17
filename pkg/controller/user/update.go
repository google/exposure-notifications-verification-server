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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleUpdate() http.Handler {
	type FormData struct {
		Name        string            `form:"name"`
		Admin       bool              `form:"admin"`
		Permissions []rbac.Permission `form:"permissions"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.UserWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		user, userMembership, err := c.findUser(currentUser, currentRealm, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderEdit(ctx, w, user, userMembership)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("failed to process form: %v", err)
			c.renderEdit(ctx, w, user, userMembership)
			return
		}

		// Update user properties.
		user.Name = form.Name
		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to update user: %v", err)
			c.renderEdit(ctx, w, user, userMembership)
			return
		}

		// Update membership properties, iff the target user differs
		if currentUser.ID != user.ID {
			permission, err := rbac.CompileAndAuthorize(membership.Permissions, form.Permissions)
			if err != nil {
				flash.Error("Failed to update user permissions: %s", err)
				c.renderEdit(ctx, w, user, userMembership)
				return
			}
			if err := user.AddToRealm(c.db, currentRealm, permission, currentUser); err != nil {
				flash.Error("Failed to update user in realm: %v", err)
				c.renderEdit(ctx, w, user, membership)
				return
			}
		}

		flash.Alert("Successfully updated user '%v'", form.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/users/%d", user.ID), http.StatusSeeOther)
	})
}

func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, user *database.User, membership *database.Membership) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Edit user: %s", user.Name)
	m["user"] = user
	m["userMembership"] = membership
	m["permissions"] = rbac.NamePermissionMap
	c.h.RenderHTML(w, "users/edit", m)
}
