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

// Package middleware defines shared middleware for handlers.
package middleware

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireAuth requires a user to be logged in. It also fetches and stores
// information about the user on the request context.
func RequireAuth(cacher cache.Cacher, authProvider auth.Provider, db *database.Database, h *render.Renderer, sessionIdleTTL, expiryCheckTTL time.Duration) mux.MiddlewareFunc {
	cacheTTL := 30 * time.Minute

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireAuth")

			session := controller.SessionFromContext(ctx)
			if session == nil {
				logger.Errorw("session does not exist")
				controller.MissingSession(w, r, h)
				return
			}
			flash := controller.Flash(session)

			// Check session idle timeout.
			if t := controller.LastActivityFromSession(session); !t.IsZero() {
				// If it's been more than the TTL since we've seen this session,
				// expire it by creating a new empty session.
				if time.Since(t) > sessionIdleTTL {
					flash.Error("Your session has expired due to inactivity.")
					controller.RedirectToLogout(w, r, h)
					return
				}
			}

			// Get the email from the auth provider.
			email, err := authProvider.EmailAddress(ctx, session)
			if err != nil {
				logger.Debugw("failed to get email from session", "error", err)
				flash.Error("An error occurred trying to verify your credentials.")
				controller.RedirectToLogout(w, r, h)
				return
			}

			// Load the user by using the cache to alleviate pressure on the database
			// layer.
			var user database.User
			userCacheKey := &cache.Key{
				Namespace: "users:by_email",
				Key:       email,
			}
			if err := cacher.Fetch(ctx, userCacheKey, &user, cacheTTL, func() (interface{}, error) {
				return db.FindUserByEmail(email)
			}); err != nil {
				authProvider.ClearSession(ctx, session)

				if database.IsNotFound(err) {
					controller.Unauthorized(w, r, h)
					return
				}

				logger.Errorw("failed to lookup user", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			// Check if the session is still valid.
			if time.Now().After(user.LastRevokeCheck.Add(expiryCheckTTL)) {
				// Check if the session has been revoked.
				if err := authProvider.CheckRevoked(ctx, session); err != nil {
					logger.Debugw("session revoked", "error", err)
					flash.Error("You have been logged out from another session.")
					controller.RedirectToLogout(w, r, h)
					return
				}

				// Update the revoke check time.
				if err := db.TouchUserRevokeCheck(&user); err != nil {
					logger.Errorw("failed to update revocation check time", "error", err)
					controller.InternalError(w, r, h, err)
					return
				}

				// Update the user in the cache so it has the new revoke check time.
				if err := cacher.Write(ctx, userCacheKey, &user, cacheTTL); err != nil {
					logger.Errorw("failed to cache user revocation check time", "error", err)
					controller.InternalError(w, r, h, err)
					return
				}
			}

			// Look up the user's memberships.
			memberships, err := user.ListMemberships(db)
			if err != nil {
				controller.InternalError(w, r, h, err)
				return
			}

			// Save the user on the context.
			ctx = controller.WithUser(ctx, &user)
			ctx = controller.WithMemberships(ctx, memberships)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireSystemAdmin requires the current user is a global administrator. It must
// come after RequireAuth so that a user is set on the context.
func RequireSystemAdmin(h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireAdminHandler")

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				controller.MissingUser(w, r, h)
				return
			}

			if !currentUser.SystemAdmin {
				logger.Debugw("user is not an admin")
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
