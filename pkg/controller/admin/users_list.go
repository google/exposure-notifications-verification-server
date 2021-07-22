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

package admin

import (
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/gorilla/mux"
)

// HandleUsersIndex renders the list of all non-system-admin users.
func (c *Controller) HandleUsersIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.BadRequest(w, r, c.h)
			return
		}

		var scopes []database.Scope
		filter := strings.TrimSpace(r.FormValue("filter"))
		switch filter {
		case "systemAdmins":
			scopes = append(scopes, database.OnlySystemAdmins())
		default:
		}

		q := r.FormValue(QueryKeySearch)
		scopes = append(scopes, database.WithUserSearch(q))

		users, paginator, err := c.db.ListUsers(pageParams, scopes...)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m.Title("Users - System Admin")
		m["users"] = users
		m["query"] = q
		m["filter"] = filter
		m["paginator"] = paginator
		c.h.RenderHTML(w, "admin/users/index", m)
	})
}

// HandleUserShow renders details about a user.
func (c *Controller) HandleUserShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// Pull the user from the id.
		user, err := c.db.FindUser(vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		memberships, err := user.ListMemberships(c.db)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m.Title("User: %s - System Admin", user.Name)
		m["user"] = user
		m["memberships"] = memberships
		c.h.RenderHTML(w, "admin/users/show", m)
	})
}
