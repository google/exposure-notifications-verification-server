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

	"go.uber.org/zap"
)

type userSaveController struct {
	config *config.ServerConfig
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
func NewSaveController(ctx context.Context, config *config.ServerConfig, db *database.Database) http.Handler {
	return &userSaveController{config, db, logging.FromContext(ctx)}
}

func (usc *userSaveController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flash := flash.FromContext(w, r)

	var form formData
	if err := controller.BindForm(w, r, &form); err != nil {
		flash.Error("Failed to process form: %v", err)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	user, err := usc.db.CreateUser(form.Email, form.Name, form.Admin, form.Disabled)
	if err != nil {
		flash.Error("Failed to create user: %v", err)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	flash.Alert("Successfully created user %q", user.Email)
	http.Redirect(w, r, "/users", http.StatusSeeOther)

	// m := html.GetTemplateMap(r)
	// m["user"] = user
	// m["flash"] = flash
	// m[csrf.TemplateTag] = csrf.TemplateField(r)
	// usc.html.Render(w, "users", m)
}
