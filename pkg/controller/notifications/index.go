// Copyright 2021 the Exposure Notifications Verification Server authors
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

package notifications

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

const (
	// QueryKeySearch is the query key where the search query exists.
	QueryKeySearch = "q"
)

func (c *Controller) HandleIndex() http.Handler {
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
		if !membership.Can(rbac.APIKeyRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm

		pageParams, err := pagination.FromRequest(r)
		if err != nil {
			controller.BadRequest(w, r, c.h)
			return
		}

		var scopes []database.Scope
		q := r.FormValue(QueryKeySearch)
		scopes = append(scopes, database.WithRealmAdminPhoneSearch(q))

		raps, paginator, err := currentRealm.ListAdminPhones(c.db, pageParams, scopes...)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if hasSMS, err := currentRealm.HasSMSConfig(c.db); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		} else if !hasSMS {
			flash.Alert("There is no SMS configuration for this realm, so any enabled notifications will not be sent via SMS.")
		}

		c.renderIndex(ctx, w, raps, paginator, q)
	})
}

// renderIndex renders the index page.
func (c *Controller) renderIndex(ctx context.Context, w http.ResponseWriter,
	raps []*database.NotificationPhone, paginator *pagination.Paginator, query string) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm Notifications and Phone Numbers")
	m["raps"] = raps
	m["paginator"] = paginator
	m["query"] = query
	c.h.RenderHTML(w, "notifications/index", m)
}
