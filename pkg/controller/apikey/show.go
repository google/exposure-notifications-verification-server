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

// HandleShow displays the API key.
func (c *Controller) HandleShow() http.Handler {
	logger := c.logger.Named("HandleShow")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		// If the API key is present, add it to the variables map and then delete it
		// from the session.
		apiKey, ok := session.Values["apiKey"]
		if ok {
			m := controller.TemplateMapFromContext(ctx)
			m["apiKey"] = apiKey
			delete(session.Values, "apiKey")
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

		c.renderShow(w, r, authApp)
	})
}

// renderShow renders the edit page.
func (c *Controller) renderShow(w http.ResponseWriter, r *http.Request, authApp *database.AuthorizedApp) {
	ctx := r.Context()
	m := controller.TemplateMapFromContext(ctx)
	m["authApp"] = authApp
	c.h.RenderHTML(w, "apikeys/show", m)
}
