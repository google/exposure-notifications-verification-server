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

package verifyapi

import (
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a controller for the verification code verification API.
type Controller struct {
	config *config.APIServerConfig
	db     *database.Database
	cacher cache.Cacher
	kms    keys.KeyManager
	h      *render.Renderer
}

func New(cfg *config.APIServerConfig, db *database.Database, cacher cache.Cacher, kms keys.KeyManager, h *render.Renderer) *Controller {
	return &Controller{
		config: cfg,
		db:     db,
		cacher: cacher,
		kms:    kms,
		h:      h,
	}
}
