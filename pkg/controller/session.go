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

package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/gin-gonic/gin"

	firebase "firebase.google.com/go"
)

var (
	ErrorUserNotFound = errors.New("user not found")
	ErrorUserDisabled = errors.New("user disabled in database")
)

type SessionHelper struct {
	db     database.Database
	config *config.Config
}

func NewSessionHelper(config *config.Config, db database.Database) *SessionHelper {
	return &SessionHelper{db, config}
}

func (s *SessionHelper) SaveSession(c *gin.Context, cookie string) {
	c.SetCookie("session", cookie, int(s.config.SessionCookieDuration.Seconds()), "/", "", false, false)
}

func (s *SessionHelper) DestroySession(c *gin.Context) {
	c.SetCookie("session", "deleted", 0, "/", "", false, false)
}

func (s *SessionHelper) LoadUserFromSession(c *gin.Context) (database.User, error) {
	cookie, err := c.Cookie("session")
	if err != nil {
		return nil, fmt.Errorf("unable to get session cookie: %w", err)
	}

	ctx := c.Request.Context()
	app, err := firebase.NewApp(ctx, s.config.FirebaseConfig())
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase app.Auth: %w", err)
	}

	token, err := client.VerifySessionCookie(ctx, cookie)
	if err != nil {
		return nil, fmt.Errorf("session verification failed: %w", err)
	}

	email, ok := token.Claims["email"]
	if !ok {
		s.DestroySession(c)
		return nil, fmt.Errorf("session dose not contain email")
	}

	user, err := s.db.LookupUser(ctx, email.(string))
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// See if we need to perform a revoke check.
	if time.Now().After(user.LastRevokeCheck().Add(s.config.SessionCookieDuration)) {
		_, err := client.VerifySessionCookieAndCheckRevoked(ctx, cookie)
		if err != nil {
			return nil, fmt.Errorf("session revoked: %w", err)
		}

		s.db.UpdateRevokeCheck(ctx, user)
	}

	if user.Disabled() {
		return user, ErrorUserDisabled
	}

	return user, nil
}
