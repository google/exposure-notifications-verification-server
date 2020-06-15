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

// Package home defines a web controller for the home page of the verification
// server. This view allows users to issue OTP codes and tie them to a diagnosis
// and test date.
package home

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type homeController struct {
	config           *config.Config
	db               *database.Database
	session          *controller.SessionHelper
	logger           *zap.SugaredLogger
	pastDaysDuration time.Duration
}

// New creates a new controller for the home page.
func New(ctx context.Context, config *config.Config, db *database.Database, session *controller.SessionHelper) controller.Controller {
	pastDaysDuration := -1 * config.AllowedTestAge

	return &homeController{
		config:           config,
		db:               db,
		session:          session,
		logger:           logging.FromContext(ctx),
		pastDaysDuration: pastDaysDuration,
	}
}

func (hc *homeController) Execute(c *gin.Context) {
	user, err := hc.session.LoadUserFromSession(c)
	if err != nil || user.Disabled {
		hc.session.RedirectToSignout(c, err, hc.logger)
		return
	}

	m := controller.NewTemplateMap(hc.config)

	// Set test date params
	now := time.Now()
	m["maxDate"] = now.Format("2006-01-02")
	m["minDate"] = now.Add(hc.pastDaysDuration).Format("2006-01-02")
	m["duration"] = hc.config.CodeDuration.String()

	m["user"] = user
	c.HTML(http.StatusOK, "home", m)
}
