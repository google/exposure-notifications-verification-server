// Copyright 2020 the Exposure Notifications Verification Server authors
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

// Package realmkeys contains web controllers for realm certificate key management.
package realmkeys

import (
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	config         *config.ServerConfig
	db             *database.Database
	h              *render.Renderer
	publicKeyCache *keyutils.PublicKeyCache

	// systemCertificateKeyManager is the key manager used for system
	// certificates. It is not used with per-realm keys.
	systemCertificateKeyManager keys.KeyManager
}

func New(cfg *config.ServerConfig, db *database.Database, systemCertificationKeyManager keys.KeyManager, publicKeyCache *keyutils.PublicKeyCache, h *render.Renderer) *Controller {
	return &Controller{
		config:         cfg,
		db:             db,
		h:              h,
		publicKeyCache: publicKeyCache,

		systemCertificateKeyManager: systemCertificationKeyManager,
	}
}
