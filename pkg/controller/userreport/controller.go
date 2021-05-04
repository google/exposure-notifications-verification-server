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

// Package userreport defines the controller for the login page.
package userreport

import (
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	memcache "github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"

	"github.com/sethvargo/go-limiter"
)

type Controller struct {
	cacher           cache.Cacher
	config           *config.RedirectConfig
	db               *database.Database
	localCache       *memcache.Cache
	limiter          limiter.Store
	smsSigner        keys.KeyManager
	h                *render.Renderer
	hostnameToRegion map[string]string

	issueController *issueapi.Controller
}

// New creates a new login controller.
func New(cacher cache.Cacher, cfg *config.RedirectConfig, db *database.Database, limiter limiter.Store, smsSigner keys.KeyManager, h *render.Renderer) (*Controller, error) {
	cfgMap, err := cfg.HostnameToRegion()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	issueController := issueapi.New(cfg, db, limiter, smsSigner, h)

	localCache, _ := memcache.New(30 * time.Second)

	return &Controller{
		cacher:           cacher,
		config:           cfg,
		db:               db,
		localCache:       localCache,
		limiter:          limiter,
		smsSigner:        smsSigner,
		h:                h,
		hostnameToRegion: cfgMap,
		issueController:  issueController,
	}, nil
}

func addError(message string, errors []string) []string {
	if len(errors) == 0 {
		return []string{message}
	}
	return append(errors, message)
}
