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

// Package session contains the controller that exchanges firebase auth tokens
// for server side session tokens.
package session

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	firebase "firebase.google.com/go"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type sessionController struct {
	config *config.Config
	db     *database.Database
	logger *zap.SugaredLogger
}

type formData struct {
	IDToken   string `form:"idToken"`
	CSRFToken string `form:"csrfToken"`
}

// New creates a new session controller. The session controller is responsible
// for accepting the firebase auth cookie information and establishing a server
// side session.
func New(ctx context.Context, config *config.Config, db *database.Database) controller.Controller {
	return &sessionController{config, db, logging.FromContext(ctx)}
}

func (ic *sessionController) Execute(c *gin.Context) {
	var form formData
	err := c.Bind(&form)
	if err != nil {
		// TODO(mikehelmick) - handle this better.
		ic.logger.Errorf("ERROR: %v", err)
	}

	// get the firebase admin client.
	ctx := c.Request.Context()
	app, err := firebase.NewApp(ctx, ic.config.FirebaseConfig())
	if err != nil {
		ic.logger.Errorf("ERROR: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		ic.logger.Errorf("ERROR: %v", err)
	}

	// Make an online call to the firebase auth to verify the token isn't revoked.
	token, err := client.VerifyIDTokenAndCheckRevoked(ctx, form.IDToken)
	if err != nil {
		ic.logger.Errorf("error verifying ID token: %v\n", err)
		c.String(http.StatusUnauthorized, "error verifying identity")
		return
	}

	email, ok := token.Claims["email"]
	if !ok {
		ic.logger.Errorf("invalid token, no email claim")
		c.String(http.StatusUnauthorized, "invalid user")
		return
	}

	user, err := ic.db.FindUser(email.(string))
	// TODO(mikehelmick) - automatically created users in disabled state (non-disabled by config)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/signout?reason=unauthorized")
		return
	}
	if user.Disabled {
		c.Redirect(http.StatusTemporaryRedirect, "/signout?reason=account disabled")
		return
	}

	expiresIn := ic.config.SessionCookieDuration
	cookie, err := client.SessionCookie(c.Request.Context(), form.IDToken, expiresIn)
	if err != nil {
		ic.logger.Errorf("failed to create session cookie: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/signout?reason=stale")
		return
	}
	c.SetCookie("session", cookie, int(expiresIn.Seconds()), "/", "", false, false)
	c.String(http.StatusOK, `{"status": "success"}`)
}
