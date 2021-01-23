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

// Package smskeys contains web controllers for realm certificate key management.
package smskeys

import (
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller has handlers HTTP actions realted to SMS signing key management.
type Controller struct {
	config         *config.ServerConfig
	db             *database.Database
	h              render.Renderer
	publicKeyCache *keyutils.PublicKeyCache
}

// New creates a new Controller
func New(ctx context.Context, config *config.ServerConfig, db *database.Database, cacher cache.Cacher, h render.Renderer) (*Controller, error) {
	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, cacher, config.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		return nil, err
	}

	return &Controller{
		config:         config,
		db:             db,
		h:              h,
		publicKeyCache: publicKeyCache,
	}, nil
}
