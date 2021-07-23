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

// Package middleware contains application specific gin middleware functions.
package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

const (
	// APIKeyHeader is the authorization header required for APIKey protected requests.
	APIKeyHeader = "X-API-Key"
)

// RequireAPIKey reads the X-API-Key header and validates it is a real
// authorized app. It also ensures currentAuthorizedApp is set in the template map.
func RequireAPIKey(cacher cache.Cacher, db *database.Database, h *render.Renderer, allowedTypes []database.APIKeyType) mux.MiddlewareFunc {
	allowedTypesMap := make(map[database.APIKeyType]struct{}, len(allowedTypes))
	for _, t := range allowedTypes {
		allowedTypesMap[t] = struct{}{}
	}

	cacheTTL := 5 * time.Minute
	lastUsedTTL := 15 * time.Minute

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireAPIKey")

			apiKey := strings.TrimSpace(r.Header.Get(APIKeyHeader))
			if apiKey == "" {
				logger.Debugw("missing API key in request")
				controller.Unauthorized(w, r, h)
				return
			}

			// Load the authorized app by using the cache to alleviate pressure on the
			// database layer.
			var authApp database.AuthorizedApp
			authAppCacheKey := &cache.Key{
				Namespace: "authorized_apps:by_api_key",
				Key:       apiKey,
			}
			if err := cacher.Fetch(ctx, authAppCacheKey, &authApp, cacheTTL, func() (interface{}, error) {
				return db.FindAuthorizedAppByAPIKey(apiKey)
			}); err != nil {
				if database.IsNotFound(err) {
					logger.Debugw("invalid api key")
					controller.Unauthorized(w, r, h)
					return
				}

				logger.Errorw("failed to lookup authorized app", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			// Verify this is an allowed type.
			if _, ok := allowedTypesMap[authApp.APIKeyType]; !ok {
				logger.Debugw("wrong request type", "got", authApp.APIKeyType, "allowed", allowedTypes)
				controller.Unauthorized(w, r, h)
				return
			}

			// Lookup the realm.
			var realm database.Realm
			realmCacheKey := &cache.Key{
				Namespace: "realms:by_id",
				Key:       strconv.FormatUint(uint64(authApp.RealmID), 10),
			}
			if err := cacher.Fetch(ctx, realmCacheKey, &realm, cacheTTL, func() (interface{}, error) {
				return authApp.Realm(db)
			}); err != nil {
				if database.IsNotFound(err) {
					logger.Warnw("realm does not exist", "id", authApp.RealmID)
					controller.Unauthorized(w, r, h)
					return
				}

				logger.Errorw("failed to lookup realm from authorized app", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			// Mark API key as used.
			if authApp.LastUsedAt == nil || time.Since(*authApp.LastUsedAt) > lastUsedTTL {
				if err := authApp.TouchLastUsedAt(db); err != nil {
					// Log an error, but do not reject the request.
					logger.Errorw("failed to update last_used_at", "error", err)
				} else {
					// Update the cache entry.
					if err := cacher.Write(ctx, authAppCacheKey, &authApp, cacheTTL); err != nil {
						logger.Errorw("failed to update cached entry for last_used_at", "error", err)
						controller.InternalError(w, r, h, err)
						return
					}
				}
			}

			// Save the authorized app on the context.
			ctx = controller.WithAuthorizedApp(ctx, &authApp)
			ctx = controller.WithRealm(ctx, &realm)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
