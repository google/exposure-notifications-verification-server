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

// Package admin contains controllers for system wide administrative actions.
package admin

import (
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/sethvargo/go-limiter"
)

type Controller struct {
	cacher       cache.Cacher
	config       *config.ServerConfig
	db           *database.Database
	authProvider auth.Provider
	h            *render.Renderer
	limiter      limiter.Store
}

func New(
	config *config.ServerConfig,
	cacher cache.Cacher,
	db *database.Database,
	authProvider auth.Provider,
	limiter limiter.Store,
	h *render.Renderer,
) *Controller {
	return &Controller{
		config:       config,
		cacher:       cacher,
		db:           db,
		authProvider: authProvider,
		h:            h,
		limiter:      limiter,
	}
}
