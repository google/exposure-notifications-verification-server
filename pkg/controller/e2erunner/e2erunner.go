// Copyright 2021 the Exposure Notifications Verification Server authors
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

// Package e2erunner implements the end-to-end runner.
package e2erunner

import (
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a controller for the e2e runner service.
type Controller struct {
	config *config.E2ERunnerConfig
	db     *database.Database
	client *clients.ENXRedirectClient
	h      *render.Renderer
}

// New creates a new cleanup controller.
func New(cfg *config.E2ERunnerConfig, db *database.Database, client *clients.ENXRedirectClient, h *render.Renderer) *Controller {
	return &Controller{
		config: cfg,
		db:     db,
		client: client,
		h:      h,
	}
}
