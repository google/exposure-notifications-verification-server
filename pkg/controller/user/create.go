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
)

func (c *Controller) HandleCreate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

		user := &database.User{}
		userMembership := &database.Membership{}

		if r.Method == http.MethodGet {
			c.renderNew(ctx, w, user, userMembership)
			return
		}

		if err := bindCreateForm(r, currentMembership, user, userMembership); err != nil {
			user.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, user, userMembership)
			return
		}

		// See if the user already exists by email - they may be a member of another
		// realm.
		existing, err := c.db.FindUserByEmail(user.Email)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}
		if existing != nil && existing.ID != 0 {
			user = existing
		}

		// Create or update user.
		if err := c.db.SaveUser(user, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNew(ctx, w, user, userMembership)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Create or update membership properties.
		if err := user.AddToRealm(c.db, currentRealm, userMembership.Permissions, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNew(ctx, w, user, userMembership)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Ensure the user exists in the upstream auth provider.
		inviteComposer, err := controller.SendInviteEmailFunc(ctx, c.db, c.h, user.Email, currentRealm)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if _, err := c.authProvider.CreateUser(ctx, user.Name, user.Email, "", true, inviteComposer); err != nil {
			user.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, user, userMembership)
			return
		}

		flash.Alert("Successfully created user %q", user.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/users/%d", user.ID), http.StatusSeeOther)
	})
}

func bindCreateForm(r *http.Request, currentMembership *database.Membership, user *database.User, membership *database.Membership) error {
	type FormData struct {
		Email       string            `form:"email"`
		Name        string            `form:"name"`
		Permissions []rbac.Permission `form:"permissions"`
	}

	var form FormData
	formErr := controller.BindForm(nil, r, &form)
	user.Email = form.Email
	user.Name = form.Name

	permissions, rbacErr := rbac.CompileAndAuthorize(currentMembership.Permissions, form.Permissions)
	membership.Permissions = permissions

	if formErr != nil {
		return formErr
	}
	return rbacErr
}

func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, user *database.User, membership *database.Membership) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("New user")
	m["user"] = user
	m["userMembership"] = membership
	m["permissions"] = rbac.NamePermissionMap
	c.h.RenderHTML(w, "users/new", m)
}
