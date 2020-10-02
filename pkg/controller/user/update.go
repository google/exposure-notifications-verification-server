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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

func (c *Controller) HandleUpdate() http.Handler {
	type FormData struct {
		Name  string `form:"name"`
		Admin bool   `form:"admin"`
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

		user, err := realm.FindUser(c.db, vars["id"])
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
			c.renderEdit(ctx, w, user)
			return
		}

		var form FormData
		err = controller.BindForm(w, r, &form)

		// Build the user struct
		user.Name = strings.TrimSpace(form.Name)
		if err != nil {
			if terr, ok := err.(schema.MultiError); ok {
				for k, err := range terr {
					user.AddError(k, err.Error())
				}
			}

			flash.Error("Failed to process form: %v", err)
			c.renderEdit(ctx, w, user)
			return
		}

		// Manage realm admin permissions.
		if form.Admin {
			user.AddRealmAdmin(realm)
		} else {
			user.RemoveRealmAdmin(realm)
		}

		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to update user: %v", err)
			c.renderUpdate(ctx, w, user)
			return
		}

		flash.Alert("Successfully updated user '%v'", form.Name)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
	})
}

func (c *Controller) renderUpdate(ctx context.Context, w http.ResponseWriter, user *database.User) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = user
	c.h.RenderHTML(w, "users/new", m)
}

func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, user *database.User) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = user
	c.h.RenderHTML(w, "users/edit", m)
}
