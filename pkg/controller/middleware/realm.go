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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/gorilla/mux"
)

// RequireRealm requires a realm to exist in the session. It also ensures the
// realm is set as currentRealm in the template map. It must come after
// RequireAuth so that a user is set on the context.
func RequireRealm(ctx context.Context, db *database.Database, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireRealm")

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

			// Lookup the realm to ensure it exists (and to cache it later).
			realm, err := db.GetRealm(realmID)
			if err != nil {
				logger.Errorw("failed to get realm", "error", err)
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

			// Save the realm in the template map.
			m := controller.TemplateMapFromContext(ctx)
			m["currentRealm"] = realm

			// Save the realm on the context.
			ctx = controller.WithRealm(ctx, realm)
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
