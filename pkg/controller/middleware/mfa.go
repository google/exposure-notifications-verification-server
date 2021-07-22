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

package middleware

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireMFA checks the realm's MFA requirements and enforces them.
// Use requireRealm before requireMFA to ensure the currently selected realm is on context.
// If no realm is selected, this assumes MFA is required.
func RequireMFA(authProvider auth.Provider, h *render.Renderer) mux.MiddlewareFunc {
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

			mfaEnabled, err := authProvider.MFAEnabled(ctx, session)
			if err != nil {
				controller.InternalError(w, r, h, err)
				return
			}

			prompted := controller.MFAPromptedFromSession(session)
			if !mfaEnabled {
				if mode := currentRealm.EffectiveMFAMode(membership.CreatedAt); mode == database.MFARequired ||
					mode == database.MFAOptionalPrompt && !prompted {
					controller.RedirectToMFA(w, r, h)
					return
				}
			}

			// Store prompted check, so we don't check again.
			controller.StoreSessionMFAPrompted(session, true)

			next.ServeHTTP(w, r)
		})
	}
}
