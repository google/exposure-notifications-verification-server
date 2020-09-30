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

// Package user contains web controllers for listing and adding users.
package user

import (
	"context"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
)

// Controller manages users
type Controller struct {
	cacher           cache.Cacher
	firebaseInternal *firebase.Client
	client           *auth.Client
	config           *config.ServerConfig
	db               *database.Database
	metrics          *controller.Metrics
	h                *render.Renderer
	logger           *zap.SugaredLogger
}

// New creates a new controller for managing users.
func New(
	ctx context.Context,
	firebaseInternal *firebase.Client,
	client *auth.Client,
	cacher cache.Cacher,
	config *config.ServerConfig,
	db *database.Database,
	h *render.Renderer) (context.Context, *Controller, error) {
	logger := logging.FromContext(ctx)
	ctx, metrics, err := controller.MetricsFromContext(ctx)
	if err != nil {
		return ctx, nil, err
	}

	return ctx, &Controller{
		cacher:           cacher,
		firebaseInternal: firebaseInternal,
		client:           client,
		config:           config,
		db:               db,
		metrics:          metrics,
		h:                h,
		logger:           logger,
	}, nil
}
