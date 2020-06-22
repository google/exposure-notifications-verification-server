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
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type userDeleteController struct {
	config *config.Config
	db     *database.Database
	logger *zap.SugaredLogger
}

// NewDeleteController creates a controller to Delete users.
func NewDeleteController(ctx context.Context, config *config.Config, db *database.Database) controller.Controller {
	return &userDeleteController{config, db, logging.FromContext(ctx)}
}

func (sc *userDeleteController) Execute(c *gin.Context) {
	// All roads lead to a GET redirect.
	defer c.Redirect(http.StatusSeeOther, "/users")
	fmt.Println("delete user: ")

	flash := flash.FromContext(c)
	email := c.Param("email")

	user, err := sc.db.FindUser(email)
	if err != nil {
		flash.Error("Failed to find user: %v", err)
		return
	}

	if err := sc.db.DeleteUser(user); err != nil {
		flash.Error("Failed to delete user: %v", err)
		return
	}

	flash.Alert("Deleted User %q", user)
}
