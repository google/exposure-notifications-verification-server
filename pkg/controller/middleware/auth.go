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
	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserDisabled = errors.New("user disabled")
)

// RequireAdmin verifies that a user is an admin. It must be used AFTER the
// RequireAuth middleware.
func RequireAdmin(ctx context.Context) gin.HandlerFunc {
	logger := logging.FromContext(ctx)

	return func(c *gin.Context) {
		if err := func() error {
			userRaw, ok := c.Get("user")
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
			logger.Errorw("RequireAdmin", "error", err)

			switch c.NegotiateFormat(gin.MIMEJSON, gin.MIMEHTML) {
			case gin.MIMEJSON:
				c.JSON(http.StatusUnauthorized, nil)
			case gin.MIMEHTML:
				flash.FromContext(c).Error("Unauthorized")
				c.Redirect(http.StatusFound, "/signout")
			}

			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAuth requires a user is authenticated using firebase auth, that such a
// user exists in the database, and that said user is not disabled.
func RequireAuth(ctx context.Context, client *auth.Client, db *database.Database, ttl time.Duration) gin.HandlerFunc {
	logger := logging.FromContext(ctx)

	return func(c *gin.Context) {
		if err := func() error {
			ctx := c.Request.Context()

			// Get the cookie
			cookie, err := c.Cookie("session")
			if err != nil {
				return fmt.Errorf("failed to get cookie: %w", err)
			}

			// Verify cookie
			token, err := client.VerifySessionCookie(ctx, cookie)
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
			user, err := db.FindUser(email)
			if err != nil || user == nil {
				return ErrUserNotFound
			}

			// Verify the user is not disabled
			if user.Disabled {
				return ErrUserDisabled
			}

			// Check if the session is still valid
			if time.Now().After(user.LastRevokeCheck.Add(ttl)) {
				if _, err := client.VerifySessionCookieAndCheckRevoked(ctx, cookie); err != nil {
					return fmt.Errorf("failed to verify session is not revoked: %w", err)
				}

				user.LastRevokeCheck = time.Now()
				if err := db.SaveUser(user); err != nil {
					return fmt.Errorf("failed to update revoke check time: %w", err)
				}
			}

			// Save the user on the context - this is how handlers access the user
			c.Set("user", user)
			return nil
		}(); err != nil {
			logger.Errorw("RequireAuth", "error", err)

			switch c.NegotiateFormat(gin.MIMEJSON, gin.MIMEHTML) {
			case gin.MIMEJSON:
				c.JSON(http.StatusUnauthorized, nil)
			case gin.MIMEHTML:
				flash.FromContext(c).Error("Unauthorized")
				c.Redirect(http.StatusFound, "/signout")
			}

			c.Abort()
			return
		}

		c.Next()
	}
}
