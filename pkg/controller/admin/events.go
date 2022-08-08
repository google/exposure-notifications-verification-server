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

package admin

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

const (
	// QueryFromSearch is the query key for a starting time.
	QueryFromSearch = "from"

	// QueryToSearch is the query key for an ending time.
	QueryToSearch = "to"

	// QueryRealmIDSearch is the query key to filter by realmID.
	QueryRealmIDSearch = "realm_id"

	// QueryIncludeTest is the query key to filter on events from test or not.
	QueryIncludeTest = "include_test"
)

// HandleEventsShow shows event logs.
func (c *Controller) HandleEventsShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// Parse query params
		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.BadRequest(w, r, c.h)
			return
		}
		from := r.FormValue(QueryFromSearch)
		to := r.FormValue(QueryToSearch)
		scopes := []database.Scope{}
		scopes = append(scopes, database.WithAuditTime(from, to))
		realmID := project.TrimSpace(r.FormValue(QueryRealmIDSearch))

		includeTest, _ := strconv.ParseBool(r.FormValue(QueryIncludeTest))
		if !includeTest {
			scopes = append(scopes, database.WithoutAuditTest())
		}

		// Add realm filter if applicable
		var realm *database.Realm
		switch realmID {
		case "":
			// All events
		case "0":
			realm = &database.Realm{
				Model: gorm.Model{ID: 0},
				Name:  "System",
			}
		default:
			realm, err = c.db.FindRealm(realmID)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// If a specific realm was provided, filter by that realm.
		if realm != nil {
			scopes = append(scopes, database.WithAuditRealmID(realm.ID))
		}

		// List the events
		events, paginator, err := c.db.ListAudits(pageParams, scopes...)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderEvents(ctx, w, events, paginator, from, to, realm)
	})
}

func (c *Controller) renderEvents(ctx context.Context, w http.ResponseWriter,
	events []*database.AuditEntry, paginator *pagination.Paginator, from, to string, realm *database.Realm,
) {
	m := controller.TemplateMapFromContext(ctx)
	m["events"] = events
	m["paginator"] = paginator
	m[QueryFromSearch] = from
	m[QueryToSearch] = to
	m["realm"] = realm
	c.h.RenderHTML(w, "admin/events/index", m)
}
