// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleCreate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
		if !membership.Can(rbac.APIKeyWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm
		currentUser := membership.User

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			var authApp database.AuthorizedApp
			authApp.APIKeyType = database.APIKeyTypeInvalid
			c.renderNew(ctx, w, &authApp)
			return
		}

		var authApp database.AuthorizedApp
		if err := bindCreateForm(r, &authApp); err != nil {
			authApp.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderNew(ctx, w, &authApp)
			return
		}

		apiKey, err := currentRealm.CreateAuthorizedApp(c.db, &authApp, currentUser)
		if err != nil {
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderNew(ctx, w, &authApp)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Store the API key on the session temporarily so it can be displayed on
		// the next page.
		session.Values["apiKey"] = apiKey

		flash.Alert("Successfully created API Key for %q", authApp.Name)
		http.Redirect(w, r, fmt.Sprintf("/realm/apikeys/%d", authApp.ID), http.StatusSeeOther)
	})
}

func bindCreateForm(r *http.Request, app *database.AuthorizedApp) error {
	type FormData struct {
		Name string              `form:"name"`
		Type database.APIKeyType `form:"type"`
	}

	var form FormData
	err := controller.BindForm(nil, r, &form)
	app.Name = form.Name
	app.APIKeyType = form.Type
	return err
}

// renderNew renders the edit page.
func (c *Controller) renderNew(ctx context.Context, w http.ResponseWriter, authApp *database.AuthorizedApp) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("New API key")
	m["authApp"] = authApp
	m["typeAdmin"] = database.APIKeyTypeAdmin
	m["typeDevice"] = database.APIKeyTypeDevice
	m["typeStats"] = database.APIKeyTypeStats
	c.h.RenderHTML(w, "apikeys/new", m)
}
