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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentMembership := controller.MembershipFromContext(ctx)
		if currentMembership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !currentMembership.Can(rbac.UserWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := currentMembership.Realm
		currentUser := currentMembership.User

		// Look up the user.
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

		if err := bindUpdateForm(r, currentMembership, user, userMembership); err != nil {
			user.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, user, userMembership)
			return
		}

		// Update user.
		if err := c.db.SaveUser(user, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNew(ctx, w, user, userMembership)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Update membership properties, iff the target user differs.
		if currentUser.ID != user.ID {
			if err := user.AddToRealm(c.db, currentRealm, userMembership.Permissions, currentUser); err != nil {
				if database.IsValidationError(err) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					c.renderNew(ctx, w, user, userMembership)
					return
				}

				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		flash.Alert("Successfully updated user %q", user.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/users/%d", user.ID), http.StatusSeeOther)
	})
}

func bindUpdateForm(r *http.Request, currentMembership *database.Membership, user *database.User, membership *database.Membership) error {
	type FormData struct {
		Name        string            `form:"name"`
		Permissions []rbac.Permission `form:"permissions"`
	}

	var form FormData
	formErr := controller.BindForm(nil, r, &form)
	user.Name = form.Name

	permissions, rbacErr := rbac.CompileAndAuthorize(currentMembership.Permissions, form.Permissions)
	membership.Permissions = permissions

	if formErr != nil {
		return formErr
	}
	return rbacErr
}

func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, user *database.User, membership *database.Membership) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Edit user: %s", user.Name)
	m["user"] = user
	m["userMembership"] = membership
	m["permissions"] = rbac.NamePermissionMap
	c.h.RenderHTML(w, "users/edit", m)
}
