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

package apikey

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

func (c *Controller) HandleDisable() http.Handler {
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
		if !membership.Can(rbac.APIKeyWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm
		currentUser := membership.User

		authApp, err := currentRealm.FindAuthorizedApp(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		now := time.Now().UTC()
		authApp.DeletedAt = &now
		if err := c.db.SaveAuthorizedApp(authApp, currentUser); err != nil {
			flash.Error("Failed to disable API Key: %v", err)
			http.Redirect(w, r, "/realm/apikeys", http.StatusSeeOther)
		}

		flash.Alert("Successfully disabled API key '%v'", authApp.Name)
		http.Redirect(w, r, "/realm/apikeys", http.StatusSeeOther)
	})
}
