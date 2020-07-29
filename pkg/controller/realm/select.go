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

// Package realm contains web controllers for selecting the effective realm.
package realm

import (
	"context"
	"net/http"
	"strconv"

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

type realmSelectController struct {
	config *config.ServerConfig
	db     *database.Database
	html   *render.HTML
	logger *zap.SugaredLogger
}

func NewSelectController(ctx context.Context, config *config.ServerConfig, db *database.Database, html *render.HTML) http.Handler {
	return &realmSelectController{config, db, html, logging.FromContext(ctx)}
}

func (rsc *realmSelectController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flash := flash.FromContext(w, r)
	user := controller.UserFromContext(ctx)
	if user == nil {
		flash.Error("Internal error, you have been logged out.")
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}

	userRealms := user.Realms
	if len(userRealms) == 0 {
		flash.Error("No realms enabled. Contact your administrator.")
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}
	if len(userRealms) == 1 {
		flash.Alert("Logged into verification system for '%v", userRealms[0].Name)
		setRealmCookie(w, rsc.config, userRealms[0].ID)
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	// Process the realm cookie if one is present, this will highlight the currently selected realm.
	var previousRealmID int64
	cookie, err := r.Cookie("realm")
	if err == nil {
		realmID, err := strconv.ParseInt(cookie.Value, 10, 64)
		if err == nil {
			previousRealmID = realmID
		}
	}

	// User must select their realm.
	m := html.GetTemplateMap(r)
	m["user"] = user
	m["realms"] = userRealms
	m["selectedRealmID"] = previousRealmID
	m[csrf.TemplateTag] = csrf.TemplateField(r)
	rsc.html.Render(w, "select-realm", m)
}
