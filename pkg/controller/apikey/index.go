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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
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

		// Perform the lazy load on authorized apps for the realm.
		if _, err := realm.GetAuthorizedApps(c.db, true); err != nil {
			flash.ErrorNow("Error loading API Keys: %v", err)
		}

		m := html.GetTemplateMap(r)
		m["user"] = user
		m["realm"] = realm
		m["apps"] = realm.AuthorizedApps
		m["flash"] = flash
		m["typeAdmin"] = database.APIUserTypeAdmin
		m["typeDevice"] = database.APIUserTypeDevice
		m[csrf.TemplateTag] = csrf.TemplateField(r)
		c.h.RenderHTML(w, "apikeys", m)
	})
}
