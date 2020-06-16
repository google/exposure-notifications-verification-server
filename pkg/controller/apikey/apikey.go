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

// Package apikey contains web controllers for listing and adding API Keys.
package apikey

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type apikeyListController struct {
	config  *config.Config
	db      *database.Database
	session *controller.SessionHelper
	logger  *zap.SugaredLogger
}

func NewListController(ctx context.Context, config *config.Config, db *database.Database, session *controller.SessionHelper) controller.Controller {
	return &apikeyListController{config, db, session, logging.FromContext(ctx)}
}

func (lc *apikeyListController) Execute(c *gin.Context) {
	user, err := lc.session.LoadUserFromSession(c)
	if err != nil || user.Disabled {
		lc.session.RedirectToSignout(c, err, lc.logger)
		return
	}
	if !user.Admin {
		lc.session.AddFlash(c, "error", "That action is not authorized.")
		c.Redirect(http.StatusTemporaryRedirect, "/home")
		return
	}

	m := controller.NewTemplateMapFromSession(lc.config, c, lc.session)
	m["user"] = user

	apps, err := lc.db.ListAuthorizedApps(true)
	if err != nil {
		m.AddError(fmt.Sprintf("Error loading API Keys: %v", err))
	} else {
		if len(apps) == 0 {
			m.AddAlert("There are no API Keys configured.")
		}
		m["apps"] = apps
	}
	c.HTML(http.StatusOK, "apikeys", m)
}
