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

// Package stats produces statistics.
package stats

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// HandleRealmUsersStats renders statistics for the current realm.
func (c *Controller) HandleRealmUsersStats(typ StatsType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		currentRealm, ok := authorizeFromContext(ctx, rbac.StatsRead)
		if !ok {
			controller.Unauthorized(w, r, c.h)
			return
		}

		stats, err := currentRealm.UserStatsCached(ctx, c.db, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		switch typ {
		case StatsTypeCSV:
			c.h.RenderCSV(w, http.StatusOK, csvFilename("user-stats"), stats)
			return
		case StatsTypeJSON:
			c.h.RenderJSON(w, http.StatusOK, stats)
			return
		default:
			controller.NotFound(w, r, c.h)
			return
		}
	})
}
