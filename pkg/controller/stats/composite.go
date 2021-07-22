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

package stats

import (
	"net/http"
	"sort"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

// HandleComposite returns composite states for realm + key server
// The key server stats may be omitted if that is not enabled
// on the realm.
func (c *Controller) HandleComposite(typ Type) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		currentRealm, ok := authorizeFromContext(ctx, rbac.StatsRead)
		if !ok {
			controller.Unauthorized(w, r, c.h)
			return
		}

		realmStats, err := currentRealm.StatsCached(ctx, c.db, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		stats := database.CompositeStats(make([]*database.CompositeDay, 0, len(realmStats)))
		statsMap := make(map[time.Time]*database.CompositeDay, len(realmStats))
		for _, rs := range realmStats {
			day := &database.CompositeDay{
				Day:        rs.Date,
				RealmStats: rs,
			}
			stats = append(stats, day)
			statsMap[rs.Date] = day
		}

		keyServerStats, err := c.db.GetKeyServerStatsCached(ctx, currentRealm.ID, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}
		if keyServerStats != nil {
			days, err := c.db.ListKeyServerStatsDaysCached(ctx, currentRealm.ID, c.cacher)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			for _, ksDay := range days {
				compDay, ok := statsMap[ksDay.Day]
				if !ok {
					// if key server has stats from a day the realm doesn't, add it in.
					compDay = &database.CompositeDay{
						Day: ksDay.Day,
					}
					stats = append(stats, compDay)
				}
				compDay.KeyServerStats = ksDay.ToResponse()
			}

			sort.Slice(stats, func(i, j int) bool {
				return stats[i].Day.Before(stats[j].Day)
			})
		}

		switch typ {
		case TypeCSV:
			c.h.RenderCSV(w, http.StatusOK, csvFilename("composite-stats"), stats)
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
