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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		user := controller.UserFromContext(ctx)
		if user == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		userRealms := user.Realms
		if len(userRealms) == 0 {
			flash.Error("No realms enabled. Contact your administrator.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		factors := 0
		if session.Values != nil {
			if f := session.Values["factorCount"]; f != nil {
				if fi, ok := f.(int); ok {
					factors = fi
				}
			}
		}

		// If the user is only a member of one realm, set that and bypass selection.
		if len(userRealms) == 1 {
			realm := userRealms[0]

			controller.StoreSessionRealm(session, realm)
			flash.Alert("Logged into verification system for '%s'", realm.Name)

			if realm.MFAMode == database.MFAOptionalPrompt && factors == 0 {
				http.Redirect(w, r, "/login/registerphone", http.StatusSeeOther)
			}

			http.Redirect(w, r, "/home", http.StatusFound)
			return
		}

		// User must select their realm.
		m := controller.TemplateMapFromContext(ctx)
		m["realms"] = userRealms
		c.h.RenderHTML(w, "realms/select", m)
	})
}
