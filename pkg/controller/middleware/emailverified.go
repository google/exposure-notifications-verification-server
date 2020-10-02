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
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"firebase.google.com/go/auth"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// RequireVerified requires a user to have verified their login email.
// MUST first run RequireAuth to populate user.
func RequireVerified(ctx context.Context, client *auth.Client, db *database.Database, h *render.Renderer, ttl time.Duration) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireVerified")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			session := controller.SessionFromContext(ctx)
			if session == nil {
				logger.Errorw("session does not exist")
				controller.MissingSession(w, r, h)
				return
			}

			flash := controller.Flash(session)

			currentUser := controller.UserFromContext(ctx)
			if currentUser == nil {
				logger.Debugw("no user found when checking email verification")
				flash.Error("Log in first to verify email.")
				controller.ClearSessionFirebaseCookie(session)
				controller.MissingUser(w, r, h)
				return
			}

			firebaseUser := controller.FirebaseUserFromContext(ctx)
			realm := controller.RealmFromContext(ctx)
			if needsEmailVerification(session, realm, firebaseUser) {
				logger.Debugw("user email not verified")
				http.Redirect(w, r, "/login/manage-account?mode=verifyEmail", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func needsEmailVerification(session *sessions.Session, realm *database.Realm, firebaseUser *auth.UserRecord) bool {
	if realm == nil || realm.EmailVerifiedMode == database.MFARequired {
		return !firebaseUser.EmailVerified
	}

	if realm.EmailVerifiedMode == database.MFAOptionalPrompt &&
		!controller.EmailVerificationPromptedFromSession(session) &&
		!firebaseUser.EmailVerified {
		return true
	}

	return false
}
