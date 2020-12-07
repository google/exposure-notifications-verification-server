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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/icsv"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

const cacheTimeout = 30 * time.Minute

func (c *Controller) HandleStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		now := time.Now().UTC()
		past := now.Add(-30 * 24 * time.Hour)

		pth := r.URL.Path
		switch {
		case strings.HasSuffix(pth, ".csv"):
			var filename string
			var stats icsv.Marshaler
			var err error

			nowFormatted := now.Format(project.RFC3339Squish)

			switch r.URL.Query().Get("scope") {
			case "external":
				filename = fmt.Sprintf("%s-external-issuer-stats.csv", nowFormatted)
				stats, err = c.getExternalIssuerStats(ctx, realm, now, past)
			case "user":
				filename = fmt.Sprintf("%s-user-stats.csv", nowFormatted)
				stats, err = c.getUserStats(ctx, realm, now, past)
			default:
				filename = fmt.Sprintf("%s-realm-stats.csv", nowFormatted)
				stats, err = c.getRealmStats(ctx, realm, now, past)
			}

			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			c.h.RenderCSV(w, http.StatusOK, filename, stats)
		case strings.HasSuffix(pth, ".json"):
			var stats json.Marshaler
			var err error

			switch r.URL.Query().Get("scope") {
			case "external":
				stats, err = c.getExternalIssuerStats(ctx, realm, now, past)
			case "user":
				stats, err = c.getUserStats(ctx, realm, now, past)
			default:
				stats, err = c.getRealmStats(ctx, realm, now, past)
			}

			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			c.h.RenderJSON(w, http.StatusOK, stats)
		default:
			// Fallback to HTML
			c.renderHTML(ctx, w, realm)
			return
		}
	})
}

func (c *Controller) renderHTML(ctx context.Context, w http.ResponseWriter, realm *database.Realm) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm stats")
	c.h.RenderHTML(w, "realmadmin/stats", m)
}

// getRealmStats returns the realm stats for a given date range.
func (c *Controller) getRealmStats(ctx context.Context, realm *database.Realm, now, past time.Time) (database.RealmStats, error) {
	var stats database.RealmStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
		return realm.Stats(c.db, past, now)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// getUserStats gets the per-user realm stats for a given date range.
func (c *Controller) getUserStats(ctx context.Context, realm *database.Realm, now, past time.Time) (database.RealmUserStats, error) {
	var userStats database.RealmUserStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:per_user",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &userStats, cacheTimeout, func() (interface{}, error) {
		return realm.UserStats(c.db, past, now)
	}); err != nil {
		return nil, err
	}
	return userStats, nil
}

// getExternalIssuerStats gets the external issuer stats for a given date range.
func (c *Controller) getExternalIssuerStats(ctx context.Context, realm *database.Realm, now, past time.Time) (database.ExternalIssuerStats, error) {
	var stats database.ExternalIssuerStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:per_external_issuer",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
		return realm.ExternalIssuerStats(c.db, past, now)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}
