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

	"go.uber.org/zap"
)

type apikeySaveController struct {
	config *config.ServerConfig
	db     *database.Database
	logger *zap.SugaredLogger
}

type formData struct {
	Name string               `form:"name"`
	Type database.APIUserType `form:"type"`
}

func NewSaveController(ctx context.Context, config *config.ServerConfig, db *database.Database) http.Handler {
	return &apikeySaveController{config, db, logging.FromContext(ctx)}
}

func (sc *apikeySaveController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// All roads lead to a GET redirect.
	defer http.Redirect(w, r, "/apikeys", http.StatusSeeOther)
	flash := flash.FromContext(w, r)

	var form formData
	if err := controller.BindForm(w, r, &form); err != nil {
		sc.logger.Errorf("invalid apikey create request: %v", err)
		flash.Error("Invalid request.")
		return
	}

	if _, err := sc.db.CreateAuthorizedApp(form.Name, form.Type); err != nil {
		sc.logger.Errorf("error creating authorized app: %v", err)
		flash.Error("Failed to create API key: %v", err)
		return
	}

	flash.Alert("Created API Key for %q", form.Name)
}
