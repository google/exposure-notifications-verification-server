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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
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
	ctx := r.Context()
	flash := flash.FromContext(w, r)

	user := controller.UserFromContext(ctx)
	if user == nil {
		flash.Error("Unauthorized")
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		flash.Error("Select realm to continue.")
		http.Redirect(w, r, "/realm", http.StatusSeeOther)
		return
	}

	m := html.GetTemplateMap(r)
	m["user"] = user

	var form formData
	if err := controller.BindForm(w, r, &form); err != nil {
		flash.Error("Failed to process form: %v", err)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	newUser, err := usc.db.FindUser(form.Email)
	if err != nil {
		// User doesn't exist, create.
		newUser, err = usc.db.CreateUser(form.Email, form.Name, false, false)
		if err != nil {
			flash.Error("Failed to create user: %v", err)
			http.Redirect(w, r, "/users", http.StatusSeeOther)
			return
		}
	}

	realm.AddUser(newUser)
	if form.Admin {
		realm.AddAdminUser(newUser)
	}

	if err := usc.db.SaveRealm(realm); err != nil {
		flash.Error("Failed to create user: %v", err)
		http.Redirect(w, r, "/users", http.StatusSeeOther)
		return
	}

	flash.Alert("Created User %v", form.Email)
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
