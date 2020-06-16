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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type apikeySaveController struct {
	config  *config.Config
	db      *database.Database
	session *controller.SessionHelper
	logger  *zap.SugaredLogger
}

type formData struct {
	Name string `form:"name"`
}

func NewSaveController(ctx context.Context, config *config.Config, db *database.Database, session *controller.SessionHelper) controller.Controller {
	return &apikeySaveController{config, db, session, logging.FromContext(ctx)}
}

func (sc *apikeySaveController) Execute(c *gin.Context) {
	user, err := sc.session.LoadUserFromSession(c)
	if err != nil || user.Disabled {
		sc.session.RedirectToSignout(c, err, sc.logger)
		return
	}

	// All roads lead to a GET redirect.
	defer c.Redirect(http.StatusSeeOther, "/apikeys")

	if !user.Admin {
		sc.session.AddError(c, "That action is not authorized.")
		return
	}

	var form formData
	if err := c.Bind(&form); err != nil {
		sc.session.AddError(c, "Invalid request.")
		return
	}

	if _, err := sc.db.CreateAuthoirzedApp(form.Name); err != nil {
		sc.session.AddError(c, fmt.Sprintf("Error creating API Key: %v", err))
		return
	}

	sc.session.AddFlash(c, "alert", fmt.Sprintf("Created API Key for '%v'", form.Name))
}
