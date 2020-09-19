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
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleShow() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		// Get and cache the stats for this user.
		var stats []*database.RealmStats
		cacheKey := fmt.Sprintf("stats:realm:%d", realm.ID)
		if err := c.cacher.Fetch(ctx, cacheKey, &stats, 5*time.Minute, func() (interface{}, error) {
			now := time.Now().UTC()
			past := now.Add(-30 * 24 * time.Hour)
			return realm.Stats(c.db, past, now)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.renderShow(ctx, w, realm, stats)
	})
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, realm *database.Realm, stats []*database.RealmStats) {
	m := controller.TemplateMapFromContext(ctx)
	m["user"] = realm
	m["stats"] = stats
	c.h.RenderHTML(w, "realmadmin/show", m)
}
