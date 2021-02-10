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

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireEmailVerified requires a user to have verified their login email.
//
// MUST first run RequireAuth to populate user and RequireRealm to populate the
// realm.
func RequireEmailVerified(authProvider auth.Provider, h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				controller.MissingSession(w, r, h)
				return
			}

			membership := controller.MembershipFromContext(ctx)
			if membership == nil {
				controller.MissingMembership(w, r, h)
				return
			}
			currentRealm := membership.Realm

			// Only try to verify email if the realm requires it.
			if currentRealm.EmailVerifiedMode == database.MFARequired ||
				(currentRealm.EmailVerifiedMode == database.MFAOptionalPrompt && !controller.EmailVerificationPromptedFromSession(session)) {
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
