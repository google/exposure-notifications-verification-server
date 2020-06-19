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

type userSaveController struct {
	config *config.Config
	db     *database.Database
	logger *zap.SugaredLogger
}

type formData struct {
	Email    string `form:"email"`
	Name     string `form:"name"`
	Admin    bool   `form:"admin"`
	Disabled bool   `form:"disabled"`
}

// NewSaveController creates a controller to save users.
func NewSaveController(ctx context.Context, config *config.Config, db *database.Database) controller.Controller {
	return &userSaveController{config, db, logging.FromContext(ctx)}
}

func (sc *userSaveController) Execute(c *gin.Context) {
	// All roads lead to a GET redirect.
	defer c.Redirect(http.StatusSeeOther, "/users")

	flash := flash.FromContext(c)

	var form formData
	if err := c.Bind(&form); err != nil {
		flash.Error("Invalid request.")
		return
	}

	if _, err := sc.db.CreateUser(form.Email, form.Name, form.Admin, form.Disabled); err != nil {
		flash.Error("Failed to create user: %v", err)
		return
	}

	flash.Alert("Created User %q", form.Name)
}
