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

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type sessionController struct {
	config *config.Config
	db     *database.Database
	client *auth.Client
	logger *zap.SugaredLogger
}

type formData struct {
	IDToken   string `form:"idToken"`
	CSRFToken string `form:"csrfToken"`
}

// New creates a new session controller. The session controller is responsible
// for accepting the firebase auth cookie information and establishing a server
// side session.
func New(ctx context.Context, config *config.Config, client *auth.Client, db *database.Database) controller.Controller {
	return &sessionController{config, db, client, logging.FromContext(ctx)}
}

func (c *sessionController) Execute(g *gin.Context) {
	ctx := g.Request.Context()
	flash := flash.FromContext(g)

	var form formData
	if err := g.Bind(&form); err != nil {
		flash.Error("Failed to process login: %v", err)
		g.JSON(http.StatusBadRequest, nil)
		return
	}

	ttl := c.config.SessionCookieDuration
	cookie, err := c.client.SessionCookie(ctx, form.IDToken, ttl)
	if err != nil {
		flash.Error("Failed to create session: %v", err)
		g.JSON(http.StatusUnauthorized, nil)
		return
	}

	g.SetCookie("session", cookie, int(ttl.Seconds()), "/", "", false, false)
	g.JSON(http.StatusOK, nil)
}
