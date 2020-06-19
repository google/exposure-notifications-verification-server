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

// Package user contains web controllers for listing and adding users.
package user

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type userListController struct {
	config *config.Config
	db     *database.Database
	logger *zap.SugaredLogger
}

// NewListController creates a controller to list users
func NewListController(ctx context.Context, config *config.Config, db *database.Database) controller.Controller {
	return &userListController{config, db, logging.FromContext(ctx)}
}

func (lc *userListController) Execute(c *gin.Context) {
	user := c.MustGet("user").(*database.User)
	flash := flash.FromContext(c)

	m := controller.NewTemplateMapFromSession(lc.config, c)
	m["user"] = user

	apps, err := lc.db.ListUsers(true)
	if err != nil {
		flash.ErrorNow("Error loading API Keys: %v", err)
	}

	m["apps"] = apps
	c.HTML(http.StatusOK, "users", m)
}
