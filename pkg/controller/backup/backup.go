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

// Package backup implements data and database backups.
package backup

import (
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

const lockName = "backupLock"

// Controller is a controller for the backup service.
type Controller struct {
	config *config.BackupConfig
	db     *database.Database
	h      *render.Renderer

	// overrideAuthToken is for testing to bypass API calls to get authentication
	// information.
	overrideAuthToken string
}

// New creates a new backup controller.
func New(cfg *config.BackupConfig, db *database.Database, h *render.Renderer) *Controller {
	return &Controller{
		config: cfg,
		db:     db,
		h:      h,
	}
}
