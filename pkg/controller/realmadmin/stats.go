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

package realmadmin

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
		if !membership.Can(rbac.StatsRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		s, err := c.db.GetKeyServerStatsCached(ctx, membership.RealmID, c.cacher)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}
		hasKeyServerStats := err == nil && s != nil

		m := controller.TemplateMapFromContext(ctx)
		m["hasKeyServerStats"] = hasKeyServerStats
		if hasKeyServerStats && membership.Can(rbac.SettingsRead) {
			m["keyServerOverride"] = s.KeyServerURLOverride
		}
		m.Title("Realm stats")
		c.h.RenderHTML(w, "realmadmin/stats", m)
	})
}
