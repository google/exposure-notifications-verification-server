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

package user

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

const statsCacheTimeout = 30 * time.Minute

func (c *Controller) HandleUserStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

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
		if !membership.Can(rbac.UserRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		// Pull the user from the id.
		user, _, err := c.findUser(currentUser, currentRealm, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		stats, err := c.getStats(ctx, user, currentRealm)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		pth := r.URL.Path
		switch {
		case strings.HasSuffix(pth, ".csv"):
			nowFormatted := time.Now().UTC().Format(project.RFC3339Squish)
			filename := fmt.Sprintf("%s-user-stats.csv", nowFormatted)
			c.h.RenderCSV(w, http.StatusOK, filename, stats)
			return
		case strings.HasSuffix(pth, ".json"):
			c.h.RenderJSON(w, http.StatusOK, stats)
			return
		default:
			controller.InternalError(w, r, c.h, fmt.Errorf("unknown path %q", pth))
			return
		}
	})
}

func (c *Controller) getStats(ctx context.Context, user *database.User, realm *database.Realm) (database.UserStats, error) {
	now := time.Now().UTC()
	past := now.Add(-30 * 24 * time.Hour)

	var stats database.UserStats
	cacheKey := &cache.Key{
		Namespace: "stats:user",
		Key:       fmt.Sprintf("%d:%d", realm.ID, user.ID),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &stats, statsCacheTimeout, func() (interface{}, error) {
		return user.Stats(c.db, realm.ID, past, now)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}
