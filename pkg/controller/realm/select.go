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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleSelect() http.Handler {
	type FormData struct {
		Realm int `form:"realm,required"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := flash.FromContext(w, r)

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			c.logger.Errorf("invalid realm select rquest: %v", err)
			flash.Error("Invalid realm selection.")
			http.Redirect(w, r, "/home/realm", http.StatusSeeOther)
			return
		}

		realm, err := c.db.GetRealm(int64(form.Realm))
		if err != nil {
			c.logger.Errorf("error selecting realm: %v", err)
			flash.Error("Invalid realm selection.")
			http.Redirect(w, r, "/home/realm", http.StatusSeeOther)
			return
		}

		user := controller.UserFromContext(ctx)
		if user == nil {
			flash.Error("Internal error, you have been logged out.")
			http.Redirect(w, r, "/signout", http.StatusSeeOther)
			return
		}

		// Verify that the user has access to the realm.
		if user.CanViewRealm(realm.ID) {
			setRealmCookie(w, c.config, realm.ID)
			flash.Alert("Selected realm '%v'", realm.Name)
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		}

		// Not allowed to see the realm selected.
		flash.Error("Invalid realm selection.")
		http.Redirect(w, r, "/home/realm", http.StatusSeeOther)
	})
}
