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
	"github.com/gorilla/mux"

	"go.uber.org/zap"
)

type userDeleteController struct {
	config *config.ServerConfig
	db     *database.Database
	logger *zap.SugaredLogger
}

// NewDeleteController creates a controller to Delete users.
func NewDeleteController(ctx context.Context, config *config.ServerConfig, db *database.Database) http.Handler {
	return &userDeleteController{config, db, logging.FromContext(ctx)}
}

func (udc *userDeleteController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer http.Redirect(w, r, "/users", http.StatusSeeOther)

	ctx := r.Context()
	flash := flash.FromContext(w, r)
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		flash.Error("Select realm to continue.")
		http.Redirect(w, r, "/realm", http.StatusSeeOther)
		return
	}

	vars := mux.Vars(r)
	email := vars["email"]

	delUser, err := udc.db.FindUser(email)
	if err != nil {
		flash.Error("Failed to find user: %v", err)
		return
	}

	if err := realm.DeleteUserFromRealm(udc.db, delUser); err != nil {
		flash.Error("Failed to delete user: %v", err)
		return
	}

	flash.Alert("Deleted User %v", delUser.Email)
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
