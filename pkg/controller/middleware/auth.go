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
	"fmt"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/httplimit"

	httpcontext "github.com/gorilla/context"

	"go.uber.org/zap"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserDisabled = errors.New("user disabled")
)

type RequireAdminHandler struct {
	logger *zap.SugaredLogger
}

// RequireAdmin verifies that a user is an admin. It must be used AFTER the
// RequireAuth middleware.
func RequireAdmin(ctx context.Context) *RequireAdminHandler {
	return &RequireAdminHandler{logging.FromContext(ctx)}
}

func (rah *RequireAdminHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			userRaw, ok := httpcontext.GetOk(r, "user")
			if !ok {
				return fmt.Errorf("missing user in session")
			}

			user, ok := userRaw.(*database.User)
			if !ok {
				return fmt.Errorf("user is not a database.User")
			}

			if !user.Admin {
				return fmt.Errorf("user is not an admin")
			}

			return nil
		}(); err != nil {
			rah.logger.Errorw("RequireAdmin", "error", err)

			if controller.IsJSONContentType(r) {
				controller.WriteJSON(w, http.StatusUnauthorized, nil)
			} else {
				flash.FromContext(w, r).Error("Unauthorized")
				http.Redirect(w, r, "/signout", http.StatusFound)
			}

			return
		}
		next.ServeHTTP(w, r)
	})
}

type RequireAuthHandler struct {
	ctx    context.Context
	client *auth.Client
	db     *database.Database
	ttl    time.Duration
	logger *zap.SugaredLogger
}

// RequireAuth requires a user is authenticated using firebase auth, that such a
// user exists in the database, and that said user is not disabled.
func RequireAuth(ctx context.Context, client *auth.Client, db *database.Database, ttl time.Duration) *RequireAuthHandler {
	logger := logging.FromContext(ctx)
	return &RequireAuthHandler{ctx, client, db, ttl, logger}
}

func (rah *RequireAuthHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			// Get the cookie
			cookie, err := r.Cookie("session")
			if err != nil {
				return fmt.Errorf("failed to get cookie: %w", err)
			}

			// Verify cookie
			token, err := rah.client.VerifySessionCookie(rah.ctx, cookie.Value)
			if err != nil {
				return fmt.Errorf("failed to verify session cookie: %w", err)
			}

			// Get the email
			emailRaw, ok := token.Claims["email"]
			if !ok {
				// s.DestroySession(c) // TODO
				return fmt.Errorf("session is missing email")
			}

			// Convert to string
			email, ok := emailRaw.(string)
			if !ok {
				return fmt.Errorf("email is not a string")
			}

			// Lookup the user by email
			user, err := rah.db.FindUser(email)
			if err != nil || user == nil {
				return ErrUserNotFound
			}

			// Verify the user is not disabled
			if user.Disabled {
				return ErrUserDisabled
			}

			// Check if the session is still valid
			if time.Now().After(user.LastRevokeCheck.Add(rah.ttl)) {
				if _, err := rah.client.VerifySessionCookieAndCheckRevoked(rah.ctx, cookie.Value); err != nil {
					return fmt.Errorf("failed to verify session is not revoked: %w", err)
				}

				user.LastRevokeCheck = time.Now()
				if err := rah.db.SaveUser(user); err != nil {
					return fmt.Errorf("failed to update revoke check time: %w", err)
				}
			}

			// Save the user on the context - this is how handlers access the user
			httpcontext.Set(r, "user", user)
			return nil
		}(); err != nil {
			rah.logger.Errorw("RequireAuth", "error", err)

			if controller.IsJSONContentType(r) {
				controller.WriteJSON(w, http.StatusUnauthorized, nil)
			} else {
				flash.FromContext(w, r).Error("Unauthorized")
				http.Redirect(w, r, "/signout", http.StatusFound)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// UserRequestLimiter requires a user is authenticated using firebase auth,
// and that said user has not made too many requests
func UserRequestLimiter(ctx context.Context, store limiter.Store) *httplimit.Middleware {
	httplimiter, err := httplimit.NewMiddleware(store, userIdKeyFunc())
	if err != nil {
		log.Fatalf("failed to create limiter middleware: %v", err)
	}
	return httplimiter
}

func userIdKeyFunc() httplimit.KeyFunc {
	return func(r *http.Request) (string, error) {
		rawUser, ok := httpcontext.GetOk(r, "user")
		if !ok {
			// flash.FromContext(w, r).Error("Unauthorized")
			// http.Redirect(w, r, "/signout", http.StatusFound)
			return "", fmt.Errorf("unauthorized")
		}
		user, ok := rawUser.(*database.User)
		if !ok {
			// flash.FromContext(w, r).Error("internal error - you have been logged out.")
			// http.Redirect(w, r, "/signout", http.StatusFound)
			return "", fmt.Errorf("internal error")
		}
		return user.Email, nil

	}
}
