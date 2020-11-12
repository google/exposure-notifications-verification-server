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
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

// HandleUpdate handles an update.
func (c *Controller) HandleUpdate() http.Handler {
	type FormData struct {
		Name string `form:"name"`
	}

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

		authApp, err := realm.FindAuthorizedApp(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderEdit(ctx, w, authApp)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			authApp.Name = form.Name

			if terr, ok := err.(schema.MultiError); ok {
				for k, err := range terr {
					authApp.AddError(k, err.Error())
				}
			}

			flash.Error("Failed to process form: %v", err)
			c.renderEdit(ctx, w, authApp)
		}

		// Build the authorized app struct
		authApp.Name = form.Name

		// Save
		if err := c.db.SaveAuthorizedApp(authApp, currentUser); err != nil {
			flash.Error("Failed to save api key: %v", err)
			c.renderEdit(ctx, w, authApp)
			return
		}

		flash.Alert("Successfully updated API key!")
		http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
	})
}

// renderEdit renders the edit page.
func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, authApp *database.AuthorizedApp) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Edit API key: %s", authApp.Name)
	m["authApp"] = authApp
	c.h.RenderHTML(w, "/realm/apikeys/edit", m)
}
