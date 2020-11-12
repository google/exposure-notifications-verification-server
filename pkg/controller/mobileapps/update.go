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

package mobileapps

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
		Name  string          `form:"name"`
		URL   string          `form:"url"`
		OS    database.OSType `form:"os"`
		AppID string          `form:"app_id"`
		SHA   string          `form:"sha"`
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

		app, err := realm.FindMobileApp(c.db, vars["id"])
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
			c.renderEdit(ctx, w, app)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			app.Name = form.Name

			if terr, ok := err.(schema.MultiError); ok {
				for k, err := range terr {
					app.AddError(k, err.Error())
				}
			}

			flash.Error("Failed to process form: %v", err)
			c.renderEdit(ctx, w, app)
		}

		// Build the authorized app struct
		app.Name = form.Name
		app.URL = form.URL
		app.OS = form.OS
		app.AppID = form.AppID
		app.SHA = form.SHA

		// Save
		if err := c.db.SaveMobileApp(app, currentUser); err != nil {
			flash.Error("Failed to save mobile app: %v", err)
			c.renderEdit(ctx, w, app)
			return
		}

		flash.Alert("Successfully updated mobile app!")
		http.Redirect(w, r, "/realm/mobile-apps", http.StatusSeeOther)
	})
}

// renderEdit renders the edit page.
func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, app *database.MobileApp) {
	m := templateMap(ctx)
	m.Title("Edit mobile app: %s", app.Name)
	m["app"] = app
	c.h.RenderHTML(w, "mobileapps/edit", m)
}
