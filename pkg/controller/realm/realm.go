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

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"
)

type Controller struct {
	config *config.ServerConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// New creates a new realm controller.
func New(ctx context.Context, config *config.ServerConfig, db *database.Database, h *render.Renderer) *Controller {
	logger := logging.FromContext(ctx)

	return &Controller{
		config: config,
		db:     db,
		h:      h,
		logger: logger,
	}
}
