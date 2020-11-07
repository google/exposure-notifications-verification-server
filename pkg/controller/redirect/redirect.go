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

// Package redirect defines the controller for the deep link redirector.
package redirect

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

type Controller struct {
	config           *config.RedirectConfig
	cacher           cache.Cacher
	db               *database.Database
	h                *render.Renderer
	hostnameToRegion map[string]string
}

// New creates a new redirect controller.
func New(ctx context.Context, db *database.Database, config *config.RedirectConfig, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	logger := logging.FromContext(ctx).Named("redirect.New")

	cfgMap, err := config.HostnameToRegion()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	logger.Infow("redirect configuration", "hostnameToRegion", cfgMap)

	return &Controller{
		config:           config,
		db:               db,
		cacher:           cacher,
		h:                h,
		hostnameToRegion: cfgMap,
	}, nil
}
