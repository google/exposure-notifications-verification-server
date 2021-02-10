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

// Package cleanup implements periodic data deletion.
package cleanup

import (
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

const cleanupName = "cleanupLock"

// Controller is a controller for the cleanup service.
type Controller struct {
	config                 *config.CleanupConfig
	db                     *database.Database
	signingTokenKeyManager keys.SigningKeyManager
	h                      *render.Renderer
}

// New creates a new cleanup controller.
func New(config *config.CleanupConfig, db *database.Database, signingTokenKeyManager keys.SigningKeyManager, h *render.Renderer) *Controller {
	return &Controller{
		config:                 config,
		db:                     db,
		signingTokenKeyManager: signingTokenKeyManager,
		h:                      h,
	}
}
