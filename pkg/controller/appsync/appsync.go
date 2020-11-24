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

// Package appsync syncs the published list of mobile apps to this server's db.
package appsync

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a controller for the appsync service.
type Controller struct {
	config *config.AppSyncConfig
	db     *database.Database
	h      *render.Renderer
}

// New creates a new appsync controller.
func New(config *config.AppSyncConfig, db *database.Database, h *render.Renderer) (*Controller, error) {
	return &Controller{
		config: config,
		db:     db,
		h:      h,
	}, nil
}

// HandleSync performs the logic to sync mobile apps.
func (c *Controller) HandleSync(ctx context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(whaught): implement this
		controller.InternalError(w, r, c.h, errors.New("not implemented"))
		return
	})
}
