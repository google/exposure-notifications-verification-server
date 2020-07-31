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

// Package session contains the controller that exchanges firebase auth tokens
// for server side session tokens.
package session

import (
	"context"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"

	"go.uber.org/zap"
)

type Controller struct {
	client   *auth.Client
	config   *config.ServerConfig
	db       *database.Database
	h        *render.Renderer
	logger   *zap.SugaredLogger
	sessions sessions.Store
}

// New creates a new session controller.
func New(ctx context.Context, client *auth.Client, config *config.ServerConfig, db *database.Database, h *render.Renderer, sessions sessions.Store) *Controller {
	logger := logging.FromContext(ctx)

	return &Controller{
		client:   client,
		config:   config,
		db:       db,
		h:        h,
		logger:   logger,
		sessions: sessions,
	}
}
