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

package realmadmin

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

type realmAdminSave struct {
	config *config.ServerConfig
	db     *database.Database
	logger *zap.SugaredLogger
}

type formData struct {
	Name string `form:"name"`
}

func NewSaveController(ctx context.Context, config *config.ServerConfig, db *database.Database) http.Handler {
	return &realmAdminSave{config, db, logging.FromContext(ctx)}
}

func (ras *realmAdminSave) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flash := flash.FromContext(w, r)

	user := controller.UserFromContext(ctx)
	if user == nil {
		http.Redirect(w, r, "/signout", http.StatusFound)
		return
	}
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		flash.Error("Select realm to continue.")
		http.Redirect(w, r, "/realm", http.StatusSeeOther)
		return
	}

	// All roads lead to a GET redirect.
	defer http.Redirect(w, r, "/settings", http.StatusSeeOther)

	var form formData
	if err := controller.BindForm(w, r, &form); err != nil {
		ras.logger.Errorf("invalid realm save request: %v", err)
		flash.Error("Invalid request.")
		return
	}

	realm.Name = form.Name
	if err := ras.db.SaveRealm(realm); err != nil {
		ras.logger.Errorf("unable save realm settings: %v", err)
		flash.Error("Error updating realm: %v", err)
		return
	}

	flash.Alert("Updated realm settings!")
}
