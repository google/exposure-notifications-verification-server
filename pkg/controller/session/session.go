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
	"time"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"go.uber.org/zap"
)

type sessionController struct {
	config *config.Config
	db     *database.Database
	client *auth.Client
	logger *zap.SugaredLogger
}

type formData struct {
	IDToken string `schema:"idToken"`
}

// New creates a new session controller. The session controller is responsible
// for accepting the firebase auth cookie information and establishing a server
// side session.
func New(ctx context.Context, config *config.Config, client *auth.Client, db *database.Database) http.Handler {
	return &sessionController{config, db, client, logging.FromContext(ctx)}
}

func (c *sessionController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flash := flash.FromContext(w, r)

	// Parse and decode form.
	var form formData
	if err := controller.BindForm(w, r, &form); err != nil {
		c.logger.Errorf("error pasring form: %v", err)
		flash.Error("Failed to process login: %v", err)
		controller.WriteJSON(w, http.StatusBadRequest, nil)
		return
	}

	ttl := c.config.SessionCookieDuration
	cookie, err := c.client.SessionCookie(ctx, form.IDToken, ttl)
	if err != nil {
		c.logger.Errorf("unable to create client session: %v", err)
		flash.Error("Failed to create session: %v", err)
		controller.WriteJSON(w, http.StatusUnauthorized, nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    cookie,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		Secure:   !c.config.DevMode,
		SameSite: http.SameSiteStrictMode,
	})
	controller.WriteJSON(w, http.StatusOK, nil)
}
