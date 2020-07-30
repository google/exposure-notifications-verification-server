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
	"net/http"
	"time"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"go.uber.org/zap"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUserDisabled  = errors.New("user disabled")
	ErrNotRealmAdmin = errors.New("not realm admin")
)

type RequireRealmAdminHandler struct {
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// RequireRealmAdmin verifies that a user is an admin in the selected realm.
// It must be used AFTER the RequireAuth and RequireRealm middlewares.
func RequireRealmAdmin(ctx context.Context, h *render.Renderer) *RequireRealmAdminHandler {
	logger := logging.FromContext(ctx)

	return &RequireRealmAdminHandler{
		h:      h,
		logger: logger,
	}
}

func (h *RequireRealmAdminHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			// Get user from context.
			user := controller.UserFromContext(r.Context())
			if user == nil {
				return fmt.Errorf("missing user in session")
			}

			// Get realm from context.
			realm := controller.RealmFromContext(r.Context())
			if realm == nil {
				return fmt.Errorf("missing realm in session")
			}

			if !user.CanAdminRealm(realm.ID) {
				return ErrNotRealmAdmin
			}

			return nil
		}(); err != nil {
			h.logger.Errorw("RequireRealmAdmin", "error", err)

			if errors.Is(err, ErrNotRealmAdmin) {
				flash.FromContext(w, r).Error("You are not authorized to admin that realm.")
				http.Redirect(w, r, "/realm", http.StatusFound)
				return
			}

			flash.FromContext(w, r).Error("Unauthorized")
			http.Redirect(w, r, "/signout", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type RequireAdminHandler struct {
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// RequireAdmin verifies that a user is a system admin.
// It must be used AFTER the RequireAuth middleware.
func RequireAdmin(ctx context.Context, h *render.Renderer) *RequireAdminHandler {
	logger := logging.FromContext(ctx)

	return &RequireAdminHandler{
		h:      h,
		logger: logger,
	}
}

func (h *RequireAdminHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := func() error {
			user := controller.UserFromContext(r.Context())
			if user == nil {
				return fmt.Errorf("missing user")
			}

			if !user.Admin {
				return fmt.Errorf("user is not an admin")
			}

			return nil
		}(); err != nil {
			h.logger.Errorw("RequireAdmin", "error", err)

			if controller.IsJSONContentType(r) {
				h.h.RenderJSON(w, http.StatusUnauthorized, nil)
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
	client *auth.Client
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger

	ttl time.Duration
}

// RequireAuth requires a user is authenticated using firebase auth, that such a
// user exists in the database, and that said user is not disabled.
func RequireAuth(ctx context.Context, client *auth.Client, db *database.Database, h *render.Renderer, ttl time.Duration) *RequireAuthHandler {
	logger := logging.FromContext(ctx)

	return &RequireAuthHandler{
		client: client,
		db:     db,
		h:      h,
		logger: logger,

		ttl: ttl,
	}
}

func (h *RequireAuthHandler) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := func() error {
			// Get the cookie
			cookie, err := r.Cookie("session")
			if err != nil {
				return fmt.Errorf("failed to get cookie: %w", err)
			}

			// Verify cookie
			token, err := h.client.VerifySessionCookie(ctx, cookie.Value)
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
			user, err := h.db.FindUser(email)
			if err != nil || user == nil {
				return ErrUserNotFound
			}

			// Verify the user is not disabled
			if user.Disabled {
				return ErrUserDisabled
			}

			// Check if the session is still valid
			if time.Now().After(user.LastRevokeCheck.Add(h.ttl)) {
				if _, err := h.client.VerifySessionCookieAndCheckRevoked(ctx, cookie.Value); err != nil {
					return fmt.Errorf("failed to verify session is not revoked: %w", err)
				}

				user.LastRevokeCheck = time.Now()
				if err := h.db.SaveUser(user); err != nil {
					return fmt.Errorf("failed to update revoke check time: %w", err)
				}
			}

			// Save the user on the request context - this is how other handlers and
			// controllers access the user.
			r = r.WithContext(controller.WithUser(r.Context(), user))
			return nil
		}(); err != nil {
			h.logger.Errorw("RequireAuth", "error", err)

			if controller.IsJSONContentType(r) {
				h.h.RenderJSON(w, http.StatusUnauthorized, nil)
			} else {
				flash.FromContext(w, r).Error("Unauthorized")
				http.Redirect(w, r, "/signout", http.StatusFound)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
