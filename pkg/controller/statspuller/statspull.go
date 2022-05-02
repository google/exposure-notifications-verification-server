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

// Package statspuller pulls statistics from the key server.
package statspuller

import (
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// Controller is a stats controller.
type Controller struct {
	defaultKeyServerClient *clients.KeyServerClient
	config                 *config.StatsPullerConfig
	db                     *database.Database
	h                      *render.Renderer
	kms                    keys.KeyManager
	signerCache            *cache.Cache[*certapi.SignerInfo]
}

// New creates a new stats-pull controller.
func New(cfg *config.StatsPullerConfig, db *database.Database, client *clients.KeyServerClient, kms keys.KeyManager, h *render.Renderer) (*Controller, error) {
	// This has to be in-memory because the signer has state and connection pools.
	signerCache, err := cache.New[*certapi.SignerInfo](cfg.CertificateSigning.SignerCacheDuration)
	if err != nil {
		return nil, fmt.Errorf("cannot create signer cache, likely invalid duration: %w", err)
	}

	return &Controller{
		config:                 cfg,
		db:                     db,
		defaultKeyServerClient: client,
		kms:                    kms,
		signerCache:            signerCache,
		h:                      h,
	}, nil
}
