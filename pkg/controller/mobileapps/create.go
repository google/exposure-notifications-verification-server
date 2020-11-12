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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		Name  string          `form:"name"`
		URL   string          `form:"url"`
		OS    database.OSType `form:"os"`
		AppID string          `form:"app_id"`
		SHA   string          `form:"sha"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var app database.MobileApp
			c.renderNew(ctx, w, &app)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			app := &database.MobileApp{
				Name:  form.Name,
				URL:   form.URL,
				OS:    form.OS,
				AppID: form.AppID,
				SHA:   form.SHA,
			}

			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w, app)
			return
		}

		// Build the authorized app struct
		app := &database.MobileApp{
			Name:    form.Name,
			RealmID: realm.ID,
			URL:     form.URL,
			OS:      form.OS,
			AppID:   form.AppID,
			SHA:     form.SHA,
		}

		if err := c.db.SaveMobileApp(app, currentUser); err != nil {
			flash.Error("Failed to create mobile app: %v", err)
			c.renderNew(ctx, w, app)
			return
		}

		flash.Alert("Successfully created mobile app '%v'", form.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/mobile-apps/%d", app.ID), http.StatusSeeOther)
	})
}

// renderNew renders the new page.
func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, app *database.MobileApp) {
	m := templateMap(ctx)
	m.Title("New mobile app")
	m["app"] = app
	c.h.RenderHTML(w, "mobileapps/new", m)
}
