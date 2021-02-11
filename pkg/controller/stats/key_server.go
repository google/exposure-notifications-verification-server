// Copyright 2021 Google LLC
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

package stats

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	keyserver "github.com/google/exposure-notifications-server/pkg/api/v1"
)

// HandleKeyServerStats renders statistics for the current realm's associate key-server.
func (c *Controller) HandleKeyServerStats(typ Type) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		currentRealm, ok := authorizeFromContext(ctx, rbac.StatsRead, rbac.UserRead)
		if !ok {
			controller.Unauthorized(w, r, c.h)
			return
		}

		days, err := c.db.ListKeyServerStatsDaysCached(ctx, currentRealm.ID, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		stats := make(keyserver.StatsDays, len(days))
		for i, d := range days {
			stats[i] = d.ToResponse()
		}

		switch typ {
		case TypeCSV:
			c.h.RenderCSV(w, http.StatusOK, csvFilename("key-server-stats"), stats)
			return
		case TypeJSON:
			c.h.RenderJSON(w, http.StatusOK, stats)
			return
		default:
			controller.NotFound(w, r, c.h)
			return
		}
	})
}
