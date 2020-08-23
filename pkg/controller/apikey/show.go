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
	"time"

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

		// Pull the authorized app from the id.
		authApp, err := realm.FindAuthorizedApp(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				logger.Debugw("auth app does not exist", "id", vars["id"])
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Get and cache the stats for this user.
		var stats []*database.AuthorizedAppStats
		cacheKey := fmt.Sprintf("stats:app:%d:%d", realm.ID, authApp.ID)
		if err := c.cacher.Fetch(ctx, cacheKey, &stats, 5*time.Minute, func() (interface{}, error) {
			now := time.Now().UTC()
			past := now.Add(-14 * 24 * time.Hour)
			return authApp.Stats(c.db, past, now)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, authApp, stats)
	})
}

// renderShow renders the edit page.
func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, authApp *database.AuthorizedApp, stats []*database.AuthorizedAppStats) {
	m := controller.TemplateMapFromContext(ctx)
	m["authApp"] = authApp
	m["stats"] = stats
	c.h.RenderHTML(w, "apikeys/show", m)
}
