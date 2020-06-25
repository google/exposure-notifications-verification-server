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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/csrf"

	"go.uber.org/zap"
)

type userListController struct {
	config *config.ServerConfig
	db     *database.Database
	html   *render.HTML
	logger *zap.SugaredLogger
}

// NewListController creates a controller to list users
func NewListController(ctx context.Context, config *config.ServerConfig, db *database.Database, html *render.HTML) http.Handler {
	return &userListController{config, db, html, logging.FromContext(ctx)}
}

func (lc *userListController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := controller.GetUser(w, r)
	if err != nil {
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}
	flash := flash.FromContext(w, r)

	lc.logger.Infof("FLASH: %+v", flash)

	m := html.GetTemplateMap(r)
	m["user"] = user

	users, err := lc.db.ListUsers(false)
	if err != nil {
		flash.ErrorNow("Error loading users: %v", err)
	}

	m["users"] = users
	m["flash"] = flash
	m[csrf.TemplateTag] = csrf.TemplateField(r)
	lc.html.Render(w, "users", m)
}
