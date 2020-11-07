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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireVerified requires a user to have verified their login email.
//
// MUST first run RequireAuth to populate user and RequireRealm to populate the
// realm.
func RequireVerified(authProvider auth.Provider, db *database.Database, h *render.Renderer, ttl time.Duration) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireVerified")

			session := controller.SessionFromContext(ctx)
			if session == nil {
				logger.Errorw("session does not exist")
				controller.MissingSession(w, r, h)
				return
			}
			flash := controller.Flash(session)

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				authProvider.ClearSession(ctx, session)

				flash.Error("Log in first to verify email.")
				controller.MissingUser(w, r, h)
				return
			}

			realm := controller.RealmFromContext(ctx)
			if realm == nil {
				controller.MissingRealm(w, r, h)
				return
			}

			// Only try to verify email if the realm requires it.
			if realm.EmailVerifiedMode == database.MFARequired ||
				(realm.EmailVerifiedMode == database.MFAOptionalPrompt && !controller.EmailVerificationPromptedFromSession(session)) {
				verified, err := authProvider.EmailVerified(ctx, session)
				if err != nil {
					controller.InternalError(w, r, h, err)
					return
				}

				if !verified {
					http.Redirect(w, r, "/login/manage-account?mode=verifyEmail", http.StatusSeeOther)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
