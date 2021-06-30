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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/gorilla/mux"
)

const (
	// QueryKeySearch is the query key where the search query exists.
	QueryKeySearch = "q"
)

// HandleMobileAppsIndex displays a list of all mobile apps.
func (c *Controller) HandleMobileAppsIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.BadRequest(w, r, c.h)
			return
		}

		q := r.FormValue(QueryKeySearch)

		apps, paginator, err := c.db.ListActiveAppsWithRealm(pageParams,
			database.WithMobileAppSearch(q))
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderMobileAppsIndex(ctx, w, apps, paginator, q)
	})
}

func (c *Controller) renderMobileAppsIndex(ctx context.Context, w http.ResponseWriter,
	apps []*database.MobileApp, paginator *pagination.Paginator, q string) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Mobile apps - System Admin")
	m["apps"] = apps
	m["paginator"] = paginator
	m["query"] = q
	c.h.RenderHTML(w, "admin/mobileapps/index", m)
}

func (c *Controller) HandleMobileAppsShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// Pull the user from the id.
		app, err := c.db.FindMobileApp(vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderMobileAppsShow(ctx, w, app)
	})
}

func (c *Controller) renderMobileAppsShow(ctx context.Context, w http.ResponseWriter, app *database.MobileApp) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("%s (%s) - Mobile apps - System Admin", app.Name, app.Realm.Name)
	m["app"] = app
	c.h.RenderHTML(w, "admin/mobileapps/show", m)
}
