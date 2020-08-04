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
)

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		Email    string `form:"email,required"`
		Name     string `form:"name,required"`
		Admin    bool   `form:"admin"`
		Disabled bool   `form:"disabled"`
	}

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

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			http.Redirect(w, r, "/users", http.StatusSeeOther)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		m["user"] = user

		newUser, err := c.db.FindUser(form.Email)
		if err != nil {
			// User doesn't exist, create.
			newUser, err = c.db.CreateUser(form.Email, form.Name, false, false)
			if err != nil {
				flash.Error("Failed to create user: %v", err)
				http.Redirect(w, r, "/users", http.StatusSeeOther)
				return
			}
		}

		realm.AddUser(newUser)
		if form.Admin {
			realm.AddAdminUser(newUser)
		}

		if err := c.db.SaveRealm(realm); err != nil {
			flash.Error("Failed to create user: %v", err)
			http.Redirect(w, r, "/users", http.StatusSeeOther)
			return
		}

		flash.Alert("Created User %v", form.Email)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
	})
}
