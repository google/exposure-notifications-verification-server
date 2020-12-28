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
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

// HandleUpdate handles an update.
func (c *Controller) HandleUpdate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.MobileAppWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		app, err := currentRealm.FindMobileApp(c.db, vars["id"])
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

		if err := bindUpdateForm(r, app); err != nil {
			app.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, app)
			return
		}

		if err := c.db.SaveMobileApp(app, currentUser); err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderEdit(ctx, w, app)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully updated mobile app %q!", app.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/mobile-apps/%d", app.ID), http.StatusSeeOther)
	})
}

func bindUpdateForm(r *http.Request, app *database.MobileApp) error {
	type FormData struct {
		Name           string          `form:"name"`
		URL            string          `form:"url"`
		EnableRedirect bool            `form:"enable_redirect"`
		OS             database.OSType `form:"os"`
		AppID          string          `form:"app_id"`
		SHA            string          `form:"sha"`
	}

	var form FormData
	err := controller.BindForm(nil, r, &form)
	app.Name = form.Name
	app.URL = form.URL
	app.DisableRedirect = !form.EnableRedirect
	app.OS = form.OS
	app.AppID = form.AppID
	app.SHA = form.SHA
	return err
}

// renderEdit renders the edit page.
func (c *Controller) renderEdit(ctx context.Context, w http.ResponseWriter, app *database.MobileApp) {
	m := templateMap(ctx)
	m.Title("Edit mobile app: %s", app.Name)
	m["app"] = app
	c.h.RenderHTML(w, "mobileapps/edit", m)
}
