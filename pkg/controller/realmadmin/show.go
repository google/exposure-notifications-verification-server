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
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var cacheTimeout = 5 * time.Minute

func (c *Controller) HandleShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		now := time.Now().UTC()
		past := now.Add(-30 * 24 * time.Hour)

		// Get and cache the stats for this realm.
		var stats []*database.RealmStats
		cacheKey := &cache.Key{
			Namespace: "stats:realm",
			Key:       strconv.FormatUint(uint64(realm.ID), 10),
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
			return realm.Stats(c.db, past, now)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Also get the per-user stats.
		var userStats []*database.RealmUserStats
		cacheKey = &cache.Key{
			Namespace: "stats:realm:per_user",
			Key:       strconv.FormatUint(uint64(realm.ID), 10),
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &userStats, cacheTimeout, func() (interface{}, error) {
			return realm.CodesPerUser(c.db, past, now)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if controller.AcceptsType(r, controller.ContentTypeJSON) {
			resp := struct {
				RealmStats []*database.RealmStats     `json:"realm_stats"`
				UserStats  []*database.RealmUserStats `json:"per_user_stats"`
			}{stats, userStats}
			c.h.RenderJSON(w, http.StatusOK, resp)
		} else {
			c.renderShow(ctx, w, realm, stats, userStats)
		}
	})
}

// formatData formats a slice of RealmUserStats into a format more conducive
// to charting in Javascript.
func formatData(userStats []*database.RealmUserStats) ([]string, [][]interface{}) {
	// We need to format the per-user-per-day data properly for the charts.
	// Create some LUTs to make this easier.
	nameLUT := make(map[string]int)
	datesLUT := make(map[time.Time]int)
	for _, stat := range userStats {
		if _, ok := nameLUT[stat.Name]; !ok {
			nameLUT[stat.Name] = len(nameLUT)
		}
		if _, ok := datesLUT[stat.Date]; !ok {
			datesLUT[stat.Date] = len(datesLUT)
		}
	}

	// Figure out the names.
	names := make([]string, len(nameLUT))
	for name, i := range nameLUT {
		names[i] = name
	}

	// And combine up the data we want to send as well.
	data := make([][]interface{}, len(datesLUT))
	for date, i := range datesLUT {
		data[i] = make([]interface{}, len(names)+1)
		data[i][0] = date.Format("Jan 2 2006")
	}
	for _, stat := range userStats {
		i := datesLUT[stat.Date]
		data[i][nameLUT[stat.Name]+1] = stat.CodesIssued
	}

	// Now, we need to format the data properly.
	return names, data
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, realm *database.Realm, stats []*database.RealmStats, userStats []*database.RealmUserStats) {
	names, format := formatData(userStats)
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = realm
	m["stats"] = stats
	m["names"] = names
	m["userStats"] = format
	c.h.RenderHTML(w, "realmadmin/show", m)
}
