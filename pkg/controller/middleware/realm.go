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

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
)

// RequireRealm requires a realm to exist in the session. It also ensures the
// realm is set as currentRealm in the template map. It must come after
// RequireAuth so that a user is set on the context.
func RequireRealm(ctx context.Context, cacher cache.Cacher, db *database.Database, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealm")

	cacheTTL := 5 * time.Minute

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			user := controller.UserFromContext(ctx)
			if user == nil {
				controller.MissingUser(w, r, h)
				return
			}

			session := controller.SessionFromContext(ctx)
			if session == nil {
				err := fmt.Errorf("session does not exist in context")
				logger.Errorw("failed to get session", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			realmID := controller.RealmIDFromSession(session)
			if realmID == 0 {
				logger.Debugw("realm does not exist in session")
				controller.MissingRealm(w, r, h)
				return
			}

			// Load the realm by using the cache to alleviate pressure on the database
			// layer.
			var realm database.Realm
			cacheKey := fmt.Sprintf("realms:by_id:%d", realmID)
			if err := cacher.Fetch(ctx, cacheKey, &realm, cacheTTL, func() (interface{}, error) {
				return db.FindRealm(realmID)
			}); err != nil {
				if database.IsNotFound(err) {
					logger.Debugw("realm does not exist")
					controller.MissingRealm(w, r, h)
					return
				}

				logger.Errorw("failed to lookup realm", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			if !user.CanViewRealm(realm.ID) {
				logger.Debugw("user cannot view realm")
				// Technically this is unauthorized, but we don't want to leak the
				// existence of a realm by returning a different error.
				controller.MissingRealm(w, r, h)
				return
			}

			if realm.MFAMode == database.MFARequired {
				if factors := controller.FactorCountFromSession(session); factors == 0 {
					http.Redirect(w, r, "/login/registerphone", http.StatusSeeOther)
				}
			}

			// Save the realm on the context.
			ctx = controller.WithRealm(ctx, &realm)
			*r = *r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRealmAdmin verifies the user is an admin of the current realm.  It
// must come after RequireAuth and RequireRealm so that a user and realm are set
// on the context.
func RequireRealmAdmin(ctx context.Context, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealmAdmin")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			user := controller.UserFromContext(ctx)
			if user == nil {
				controller.MissingUser(w, r, h)
				return
			}

			realm := controller.RealmFromContext(ctx)
			if realm == nil {
				controller.MissingRealm(w, r, h)
				return
			}

			if !user.CanAdminRealm(realm.ID) {
				logger.Debugw("user cannot manage realm")
				// Technically this is unauthorized, but we don't want to leak the
				// existence of a realm by returning a different error.
				controller.MissingRealm(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
