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

package home

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type homeController struct {
	config  *config.Config
	db      database.Database
	session *controller.SessionHelper
	logger  *zap.SugaredLogger
}

// New creates a new controlelr for the home page.
func New(ctx context.Context, config *config.Config, db database.Database, session *controller.SessionHelper) controller.Controller {
	return &homeController{config, db, session, logging.FromContext(ctx)}
}

func (hc *homeController) Execute(c *gin.Context) {
	user, err := hc.session.LoadUserFromSession(c)
	if err != nil {
		hc.logger.Errorf("invalid session: %v", err)
		reason := "unauthorized"
		if err == controller.ErrorUserDisabled {
			reason = "account disabled"
		}
		c.Redirect(http.StatusFound, "/signout?reason="+reason)
		return
	}

	m := controller.TemplateMap{}
	m["user"] = user
	c.HTML(http.StatusOK, "home", m)
}
