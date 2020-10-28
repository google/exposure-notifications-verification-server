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
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
)

// HandleStats returns an http handler for sending JSON encoded per-user stats.
func (c *Controller) HandleStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		date, err := time.Parse("2006-01-02", vars["date"])
		if err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, err)
			return
		}

		// Also get the per-user stats.
		var stats []*database.RealmUserStats
		cacheKey := &cache.Key{
			Namespace: "stats:realm:per_user",
			Key:       vars["date"],
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
			return realm.CodesPerUser(c.db, date, date)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, stats)
	})
}
