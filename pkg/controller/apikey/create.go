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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		Name string               `form:"name,required"`
		Type database.APIUserType `form:"type,required"`
	}

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

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			c.logger.Errorf("invalid apikey create request: %v", err)
			flash.Error("Invalid request.")
			http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
			return
		}

		if _, err := c.db.CreateAuthorizedApp(realm.ID, form.Name, form.Type); err != nil {
			c.logger.Errorf("error creating authorized app: %v", err)
			flash.Error("Failed to create API key: %v", err)
			http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
			return
		}

		flash.Alert("Created API Key for %q", form.Name)
		http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
	})
}
