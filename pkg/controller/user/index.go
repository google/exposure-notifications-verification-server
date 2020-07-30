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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/gorilla/csrf"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := flash.FromContext(w, r)

		user := controller.UserFromContext(ctx)
		if user == nil {
			flash.Error("Unauthorized.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			flash.Error("Select a realm to continue.")
			http.Redirect(w, r, "/realm", http.StatusSeeOther)
			return
		}

		if err := realm.LoadRealmUsers(c.db, false); err != nil {
			flash.ErrorNow("Failed to load users: %v", err)
		}
		admins := make(map[uint]bool)
		for _, au := range realm.RealmAdmins {
			admins[au.ID] = true
		}

		creationCounts := make(map[uint]int64)
		for _, user := range realm.RealmUsers {
			count, err := c.db.CountVerificationCodesByUser(user.ID)
			if err != nil {
				flash.Error("Error loading user code creation counts: %v", err)
			}

			creationCounts[user.ID] = count
		}

		m := controller.TemplateMapFromContext(ctx)
		m["user"] = user
		m["realm"] = realm
		m["admins"] = admins
		m["users"] = realm.RealmUsers
		m["codesGenerated"] = creationCounts
		m["flash"] = flash
		m[csrf.TemplateTag] = csrf.TemplateField(r)
		c.h.RenderHTML(w, "users", m)
	})
}
