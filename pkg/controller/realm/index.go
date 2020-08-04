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

package realm

import (
	"net/http"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := flash.FromContext(w, r)

		session, err := c.sessions.Get(r, "session")
		if err != nil {
			c.logger.Errorw("failed to get session", "error", err)
			flash.Error("Internal error, you have been logged out.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		user := controller.UserFromContext(ctx)
		if user == nil {
			flash.Error("Internal error, you have been logged out.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		userRealms := user.Realms
		if len(userRealms) == 0 {
			flash.Error("No realms enabled. Contact your administrator.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		// If the user is only a member of one realm, set that and bypass selection.
		if len(userRealms) == 1 {
			session.Values["realm"] = userRealms[0].ID

			if err := session.Save(r, w); err != nil {
				c.logger.Errorw("failed to save session", "error", err)
				flash.Error("Internal error, you have been logged out.")
				http.Redirect(w, r, "/signout", http.StatusSeeOther)
				return
			}

			flash.Alert("Logged into verification system for '%v", userRealms[0].Name)
			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}

		// Process the realm cookie if one is present, this will highlight the currently selected realm.
		var previousRealmID int64
		cookie, err := r.Cookie("realm")
		if err == nil {
			realmID, err := strconv.ParseInt(cookie.Value, 10, 64)
			if err == nil {
				previousRealmID = realmID
			}
		}

		// User must select their realm.
		m := controller.TemplateMapFromContext(ctx)
		m["user"] = user
		m["realms"] = userRealms
		m["selectedRealmID"] = previousRealmID
		c.h.RenderHTML(w, "select-realm", m)
	})
}
