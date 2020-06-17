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

package apikey

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

type apikeySaveController struct {
	config *config.Config
	db     *database.Database
	logger *zap.SugaredLogger
}

type formData struct {
	Name string `form:"name"`
}

func NewSaveController(ctx context.Context, config *config.Config, db *database.Database) controller.Controller {
	return &apikeySaveController{config, db, logging.FromContext(ctx)}
}

func (sc *apikeySaveController) Execute(c *gin.Context) {
	// All roads lead to a GET redirect.
	defer c.Redirect(http.StatusSeeOther, "/apikeys")

	flash := flash.FromContext(c)

	var form formData
	if err := c.Bind(&form); err != nil {
		flash.Error("Invalid request.")
		return
	}

	if _, err := sc.db.CreateAuthoirzedApp(form.Name); err != nil {
		flash.Error("Failed to create API key: %v", err)
		return
	}

	flash.Alert("Created API Key for %q", form.Name)
}
