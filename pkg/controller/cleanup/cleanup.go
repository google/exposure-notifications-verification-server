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

// Package cleanup implements periodic data deletion.
package cleanup

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
)

// Controller is a controller for the cleanup service.
type Controller struct {
	config *config.CleanupConfig
	cache  *cache.Cache
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// New creates a new cleanup controller.
func New(ctx context.Context, config *config.CleanupConfig, cache *cache.Cache, db *database.Database, h *render.Renderer) *Controller {
	logger := logging.FromContext(ctx)
	return &Controller{
		config: config,
		cache:  cache,
		db:     db,
		h:      h,
		logger: logger,
	}
}

func (c *Controller) shouldCleanup() error {
	cStatCache, err := c.cache.WriteThruLookup(database.CleanupName,
		func() (interface{}, error) {
			cState, err := c.db.FindCleanupStatus(database.CleanupName)
			if err != nil {
				return nil, err
			}
			return cState, err
		})
	if err != nil {
		// in case this was not found, create a new record.
		cStatCache, err = c.db.CreateCleanup(database.CleanupName)
		if err != nil {
			return fmt.Errorf("error attempting to backfill cleanup config: %v", err)
		}
	}

	cStat, ok := cStatCache.(*database.CleanupStatus)
	if !ok {
		return fmt.Errorf("cleanup cache is typed incorrectly")
	}

	if cStat.NotBefore.After(time.Now().UTC()) {
		return fmt.Errorf("skipping cleanup. no cleanup before %v", cStat.NotBefore)
	}

	// Attempt to advance the generation.
	cStat, err = c.db.ClaimCleanup(cStat, c.config.CleanupPeriod)
	if err != nil {
		return fmt.Errorf("unable to lock cleanup: %v", err)
	}
	if err := c.cache.Set(database.CleanupName, cStat); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}
	return nil
}

func (c *Controller) HandleCleanup() http.Handler {
	type CleanupResult struct {
		Cleanup bool `json:"cleanup"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := c.shouldCleanup(); err != nil {
			c.logger.Errorf("shouldCleanUp: %v", err)
			c.h.RenderJSON(w, http.StatusOK, &CleanupResult{false})
			return
		}

		// Cleanup tasks
		if count, err := c.db.PurgeVerificationCodes(c.config.VerificationCodeMaxAge); err != nil {
			c.logger.Errorf("db.PurgeVerificationCodes: %v", err)
		} else {
			c.logger.Infof("purged %v verification codes", count)
		}

		if count, err := c.db.PurgeTokens(c.config.VerificationTokenMaxAge); err != nil {
			c.logger.Errorf("db.PurgeTokens: %v", err)
		} else {
			c.logger.Infof("purged %v verification tokens", count)
		}

		c.h.RenderJSON(w, http.StatusOK, &CleanupResult{true})
	})
}
