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

// Package codes defines a web controller for the code status page of the
// verification server. This view allows users to view the status of
// previously-issued OTP codes.
package codes

import (
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	serverconfig *config.ServerConfig
	apiconfig    *config.AdminAPIServerConfig
	db           *database.Database
	h            render.Renderer
}

// NewServer creates a new controller for serving admin server requests.
func NewServer(ctx context.Context, config *config.ServerConfig, db *database.Database, h render.Renderer) *Controller {
	return &Controller{
		serverconfig: config,
		db:           db,
		h:            h,
	}
}

// NewAPI creates a new controller serving API requests.
func NewAPI(ctx context.Context, config *config.AdminAPIServerConfig, db *database.Database, h render.Renderer) *Controller {
	return &Controller{
		apiconfig: config,
		db:        db,
		h:         h,
	}
}
