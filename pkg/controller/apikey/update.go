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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

// HandleEdit renders the edit form.
func (c *Controller) HandleEdit() http.Handler {
	logger := c.logger.Named("HandleEdit")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		authApp, err := realm.FindAuthorizedAppString(vars["id"])
		if err != nil {
			logger.Errorw("failed to find authorized apps", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}
		if authApp == nil {
			controller.Unauthorized(w, r, c.h)
			return
		}

		c.renderEdit(w, r, authApp)
	})
}

// HandleUpdate handles an update.
func (c *Controller) HandleUpdate() http.Handler {
	type FormData struct {
		Name string `form:"name,required"`
	}

	logger := c.logger.Named("HandleUpdate")

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

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			var authApp database.AuthorizedApp
			authApp.Name = form.Name

			flash.Error("Failed to process form: %v", err)
			c.renderEdit(w, r, &authApp)
			return
		}

		authApp, err := realm.FindAuthorizedAppString(vars["id"])
		if err != nil {
			logger.Errorw("failed to find authorized apps", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}
		if authApp == nil {
			controller.Unauthorized(w, r, c.h)
			return
		}

		authApp.Name = form.Name
		if err := c.db.SaveAuthorizedApp(authApp); err != nil {
			logger.Errorw("failed to save authorized app", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully updated API key!")
		http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
	})
}

// renderEdit renders the edit page.
func (c *Controller) renderEdit(w http.ResponseWriter, r *http.Request, authApp *database.AuthorizedApp) {
	ctx := r.Context()
	m := controller.TemplateMapFromContext(ctx)
	m["authApp"] = authApp
	c.h.RenderHTML(w, "apikeys/edit", m)
}
