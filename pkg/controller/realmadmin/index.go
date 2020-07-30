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

// Package realmadmin contains web controllers for changing realm settings.
package realmadmin

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

type realmAdminView struct {
	config *config.ServerConfig
	db     *database.Database
	html   *render.HTML
	logger *zap.SugaredLogger
}

func NewViewController(ctx context.Context, config *config.ServerConfig, db *database.Database, html *render.HTML) http.Handler {
	return &realmAdminView{config, db, html, logging.FromContext(ctx)}
}

func (rav *realmAdminView) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	m["flash"] = flash
	m[csrf.TemplateTag] = csrf.TemplateField(r)
	rav.html.Render(w, "realm", m)
}
