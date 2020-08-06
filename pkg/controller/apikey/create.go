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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleNew() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var authApp database.AuthorizedApp
		authApp.APIKeyType = -1
		c.renderNew(ctx, w, &authApp)
	})
}

func (c *Controller) HandleCreate() http.Handler {
	type FormData struct {
		Name string               `form:"name,required"`
		Type database.APIUserType `form:"type,required"`
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

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			var authApp database.AuthorizedApp
			authApp.Name = form.Name
			authApp.APIKeyType = form.Type

			flash.Error("Failed to process form: %v", err)
			c.renderNew(ctx, w, &authApp)
			return
		}

		apiKey, authApp, err := c.db.CreateAuthorizedApp(realm.ID, form.Name, form.Type)
		if err != nil {
			var authApp database.AuthorizedApp
			authApp.Name = form.Name
			authApp.APIKeyType = form.Type

			c.logger.Errorw("failed to create authorized app", "error", err)
			flash.Error("Failed to create API key: %v", err)
			c.renderNew(ctx, w, &authApp)
			return
		}

		// Store the API key on the session temporarily so it can be displayed on
		// the next page.
		session.Values["apiKey"] = apiKey

		flash.Alert("Successfully created API Key for %v", form.Name)
		http.Redirect(w, r, fmt.Sprintf("/apikeys/%d", authApp.ID), http.StatusSeeOther)
	})
}

// renderNew renders the edit page.
func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, authApp *database.AuthorizedApp) {
	m := controller.TemplateMapFromContext(ctx)
	m["authApp"] = authApp
	m["typeAdmin"] = database.APIUserTypeAdmin
	m["typeDevice"] = database.APIUserTypeDevice
	c.h.RenderHTML(w, "apikeys/new", m)
}
