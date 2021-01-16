// Copyright 2021 Google LLC
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

// Package statspuller pulls statistics from the key server.
package statspuller

import (
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a stats controller.
type Controller struct {
	defaultKeyServerClient *clients.KeyServerClient
	config                 *config.StatsPullerConfig
	db                     *database.Database
	h                      render.Renderer
}

// New creates a new stats-pull controller.
func New(cfg *config.StatsPullerConfig, db *database.Database, h render.Renderer) (*Controller, error) {
	client, err := clients.NewKeyServerClient(
		cfg.KeyServerURL,
		cfg.KeyServerAPIKey,
		clients.WithTimeout(cfg.Timeout),
		clients.WithMaxBodySize(cfg.FileSizeLimitBytes))
	if err != nil {
		return nil, err
	}

	return &Controller{
		defaultKeyServerClient: client,
		config:                 cfg,
		db:                     db,
		h:                      h,
	}, nil
}
