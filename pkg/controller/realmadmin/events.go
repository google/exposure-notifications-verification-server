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

package realmadmin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
)

const (
	// QueryFromSearch is the query key for a starting time.
	QueryFromSearch = "from"

	// QueryToSearch is the query key for an ending time.
	QueryToSearch = "to"
)

func (c *Controller) HandleEvents() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var scopes []database.Scope
		from := r.FormValue(QueryFromSearch)
		to := r.FormValue(QueryToSearch)
		scopes = append(scopes, database.WithAuditSearch(from, to))

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		events, paginator, err := realm.ListAudits(c.db, pageParams, scopes...)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderEvents(ctx, w, realm, events, paginator, from, to)
	})
}

func (c *Controller) renderEvents(ctx context.Context, w http.ResponseWriter,
	realm *database.Realm, events []*database.AuditEntry, paginator *pagination.Paginator, from, to string) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = realm
	m["events"] = events
	m["paginator"] = paginator
	m[QueryFromSearch] = from
	m[QueryToSearch] = to
	c.h.RenderHTML(w, "realmadmin/events", m)
}
