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
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"firebase.google.com/go/auth"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// RequireAuth requires a user to be logged in. It also ensures that currentUser
// is set in the template map. It fetches a user from the session and stores the
// full record in the request context.
func RequireAuth(ctx context.Context, client *auth.Client, db *database.Database, h *render.Renderer, ttl time.Duration) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireAuth")

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

			firebaseCookie := controller.FirebaseCookieFromSession(session)
			if firebaseCookie == "" {
				logger.Debugw("firebase cookie not in session")
				flash.Error("An error occurred trying to verify your credentials.")
				controller.Unauthorized(w, r, h)
				return
			}

			token, err := client.VerifySessionCookie(ctx, firebaseCookie)
			if err != nil {
				logger.Debugw("failed to verify firebase cookie", "error", err)
				flash.Error("An error occurred trying to verify your credentials.")
				controller.ClearSessionFirebaseCookie(session)
				controller.Unauthorized(w, r, h)
				return
			}

			emailRaw, ok := token.Claims["email"]
			if !ok {
				logger.Debugw("firebase token does not have an email")
				flash.Error("An error occurred trying to verify your credentials.")
				controller.ClearSessionFirebaseCookie(session)
				controller.Unauthorized(w, r, h)
				return
			}

			email, ok := emailRaw.(string)
			if !ok {
				logger.Debugw("firebase email is not a string")
				flash.Error("An error occurred trying to verify your credentials.")
				controller.ClearSessionFirebaseCookie(session)
				controller.Unauthorized(w, r, h)
				return
			}

			user, err := db.FindUserByEmail(email)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logger.Debugw("user does not exist")
					flash.Error("That user does not exist.")
					controller.ClearSessionFirebaseCookie(session)
					controller.Unauthorized(w, r, h)
					return
				}

				logger.Errorw("failed to find user", "error", err)
				controller.ClearSessionFirebaseCookie(session)
				controller.InternalError(w, r, h, err)
				return
			}

			if user == nil {
				logger.Debugw("user does not exist")
				controller.ClearSessionFirebaseCookie(session)
				controller.Unauthorized(w, r, h)
				return
			}

			// Check if the session is still valid.
			if time.Now().After(user.LastRevokeCheck.Add(ttl)) {
				if _, err := client.VerifySessionCookieAndCheckRevoked(ctx, firebaseCookie); err != nil {
					logger.Debugw("failed to verify firebase cookie revocation", "error", err)
					controller.ClearSessionFirebaseCookie(session)
					controller.Unauthorized(w, r, h)
					return
				}

				user.LastRevokeCheck = time.Now()
				if err := db.SaveUser(user); err != nil {
					logger.Errorw("failed to update revocation check time", "error", err)
					controller.InternalError(w, r, h, err)
					return
				}
			}

			// Save the user in the template map.
			m := controller.TemplateMapFromContext(ctx)
			m["currentUser"] = user

			// Save the user on the context.
			ctx = controller.WithUser(ctx, user)
			*r = *r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin requires the current user is a global administrator. It must
// come after RequireAuth so that a user is set on the context.
func RequireAdmin(ctx context.Context, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireAdminHandler")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			user := controller.UserFromContext(ctx)
			if user == nil {
				controller.MissingUser(w, r, h)
				return
			}

			if !user.Admin {
				logger.Debugw("user is not an admin")
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
