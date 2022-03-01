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
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	"github.com/gorilla/mux"
)

// HandleShow displays the API key.
func (c *Controller) HandleShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		logger := logging.FromContext(ctx).Named("apikey.HandleShow")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.APIKeyRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm

		// Pull the authorized app from the id.
		authApp, err := currentRealm.FindAuthorizedApp(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				logger.Debugw("auth app does not exist", "id", vars["id"])
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// If the API key is present, add it to the variables map and then delete it
		// from the session.
		apiKey := controller.APIKeyFromSession(session)
		if apiKey != "" {
			logger.Debug("found API key in session, adding to template map")

			controller.ClearSessionAPIKey(session)
			m := controller.TemplateMapFromContext(ctx)
			m["apiKey"] = apiKey
			ctx = controller.WithTemplateMap(ctx, m)
		}

		c.renderShow(ctx, w, authApp)
	})
}

// renderShow renders the edit page.
func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, authApp *database.AuthorizedApp) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("API key: %s", authApp.Name)
	m["authApp"] = authApp
	c.h.RenderHTML(w, "apikeys/show", m)
}
