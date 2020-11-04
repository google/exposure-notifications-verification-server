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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleDelete() http.Handler {
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

		user, err := c.findUser(currentUser, realm, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Do not allow users to remove themselves. System admins can remove
		// themselves from the system admin panel.
		if user.ID == currentUser.ID {
			flash.Error("Failed to remove user from realm: cannot remove self")
			http.Redirect(w, r, "/users", http.StatusSeeOther)
			return
		}

		user.RemoveRealm(realm)

		if err := c.db.SaveUser(user, currentUser); err != nil {
			flash.Error("Failed to remove user from realm: %v", err)
			http.Redirect(w, r, "/users", http.StatusSeeOther)
			return
		}

		// If the user removed themselves from a realm, clear it from the session to
		// avoid a weird redirect.
		if user.ID == currentUser.ID {
			flash.Alert("Successfully removed you from the realm")
			controller.ClearSessionRealm(session)
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		}

		flash.Alert("Successfully removed user %v from realm", user.Email)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
	})
}
