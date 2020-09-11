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
	"github.com/gorilla/sessions"
)

// RequireMFA checks the realm's MFA requirements and enforces them.
// If no realm is selected, this assumes MFA is required.
func RequireMFA(ctx context.Context, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireMFA")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				err := fmt.Errorf("session does not exist in context")
				logger.Errorw("failed to get session", "error", err)
				controller.InternalError(w, r, h, err)
				return
			}

			realm := controller.RealmFromContext(ctx)
			if NeedsMFARedirect(session, realm) {
				controller.RedirectToMFA(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func NeedsMFARedirect(session *sessions.Session, realm *database.Realm) bool {
	if (realm.MFAMode == database.MFARequired) && controller.FactorCountFromSession(session) == 0 {
		return true
	}

	if realm.MFAMode == database.MFAOptionalPrompt && !controller.MFAPromptedFromSession(session) &&
		controller.FactorCountFromSession(session) == 0 {
		return true
	}

	return false
}
