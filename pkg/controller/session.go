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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	firebase "firebase.google.com/go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	ErrorUserNotFound = errors.New("user not found")
	ErrorUserDisabled = errors.New("user disabled in database")
)

type SessionHelper struct {
	db     *database.Database
	config *config.Config
}

type Flash map[string]string

func (s *SessionHelper) AddError(c *gin.Context, value string) error {
	return s.AddFlash(c, "error", value)
}

func (s *SessionHelper) AddFlash(c *gin.Context, name, value string) error {
	flash := s.Flash(c)
	flash[name] = value

	asBytes, err := json.Marshal(flash)
	if err != nil {
		return err
	}
	cookie := base64.StdEncoding.EncodeToString(asBytes)
	c.SetCookie("flash", cookie, 30, "/", "", false, true)
	return nil
}

func (s *SessionHelper) Flash(c *gin.Context) Flash {
	defer c.SetCookie("flash", "", 30, "/", "", false, true)
	cookie, err := c.Cookie("flash")
	if err != nil && err != http.ErrNoCookie {
		return Flash{}
	}
	if cookie == "" {
		return Flash{}
	}
	asStr, err := base64.StdEncoding.DecodeString(cookie)
	if err != nil {
		return Flash{}
	}
	var flash Flash
	if err := json.Unmarshal([]byte(asStr), &flash); err != nil {
		return Flash{}
	}
	return flash
}

func NewSessionHelper(config *config.Config, db *database.Database) *SessionHelper {
	return &SessionHelper{db, config}
}

func (s *SessionHelper) SaveSession(c *gin.Context, cookie string) {
	c.SetCookie("session", cookie, int(s.config.SessionCookieDuration.Seconds()), "/", "", false, false)
}

func (s *SessionHelper) DestroySession(c *gin.Context) {
	// negative time deletes a cookie.
	c.SetCookie("session", "deleted", -1, "/", "", false, false)
}

func (s *SessionHelper) RedirectToSignout(c *gin.Context, err error, logger *zap.SugaredLogger) {
	logger.Errorf("invalid session: %v", err)
	reason := "unauthorized"
	if err == ErrorUserDisabled {
		reason = "account disabled"
	}
	s.AddFlash(c, "error", reason)
	c.Redirect(http.StatusFound, "/signout")
}

func (s *SessionHelper) LoadUserFromSession(c *gin.Context) (*database.User, error) {
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

	user, err := s.db.FindUser(email.(string))
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// See if we need to perform a revoke check.
	if time.Now().After(user.LastRevokeCheck.Add(s.config.SessionCookieDuration)) {
		_, err := client.VerifySessionCookieAndCheckRevoked(ctx, cookie)
		if err != nil {
			return nil, fmt.Errorf("session revoked: %w", err)
		}

		user.LastRevokeCheck = time.Now()
		if err := s.db.SaveUser(user); err != nil {
			return nil, fmt.Errorf("error updating revoke check time: %w", err)
		}
	}

	if user.Disabled {
		return user, ErrorUserDisabled
	}

	return user, nil
}
