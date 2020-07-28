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
	ctx := r.Context()
	user := controller.UserFromContext(ctx)
	if user == nil {
		http.Redirect(w, r, "/signout", http.StatusSeeOther)
		return
	}

	flash := flash.FromContext(w, r)
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		flash.Error("Select realm to continue.")
		http.Redirect(w, r, "/realm", http.StatusSeeOther)
		return
	}

	m := html.GetTemplateMap(r)
	m["user"] = user
	m["realm"] = realm

	if err := realm.LoadRealmUsers(lc.db, false); err != nil {
		flash.ErrorNow("Error loading users: %v", err)
	}

	admins := make(map[uint]bool)
	for _, au := range realm.RealmAdmins {
		admins[au.ID] = true
	}

	m["admins"] = admins
	m["users"] = realm.RealmUsers
	m["flash"] = flash
	m[csrf.TemplateTag] = csrf.TemplateField(r)
	lc.html.Render(w, "users", m)
}
