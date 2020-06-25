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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"

	"github.com/google/exposure-notifications-server/pkg/cache"

	"go.uber.org/zap"
)

// Controller is a controller for the cleanup service.
type Controller struct {
	config *config.CleanupConfig
	cache  *cache.Cache
	db     *database.Database
	logger *zap.SugaredLogger
}

// New creates a new IssueAPI controller.
func New(ctx context.Context, config *config.CleanupConfig, cache *cache.Cache, db *database.Database) http.Handler {
	return &Controller{config, cache, db, logging.FromContext(ctx)}
}

func (cc *Controller) shouldCleanup() error {
	cStatCache, err := cc.cache.WriteThruLookup(database.CleanupName,
		func() (interface{}, error) {
			cState, err := cc.db.FindCleanupStatus(database.CleanupName)
			if err != nil {
				return nil, err
			}
			return cState, err
		})
	if err != nil {
		// in case this was not found, create a new record.
		cStatCache, err = cc.db.CreateCleanup(database.CleanupName)
		if err != nil {
			return fmt.Errorf("error attempting to backfil cleanup config: %v", err)
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
	cStat, err = cc.db.ClaimCleanup(cStat, cc.config.CleanupPeriod)
	if err != nil {
		return fmt.Errorf("unable to lock cleanup: %v", err)
	}
	cc.cache.Set(database.CleanupName, cStat)
	return nil
}

type cleanupResult struct {
	Cleanup bool `json:"cleanup"`
}

func (cc *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := cc.shouldCleanup(); err != nil {
		cc.logger.Errorf("shouldCleanUp: %v", err)
		controller.WriteJSON(w, http.StatusOK, &cleanupResult{false})
		return
	}

	if count, err := cc.db.PurgeVerificationCodes(cc.config.VerificationCodeMaxAge); err != nil {
		cc.logger.Errorf("db.PurgeVerificationCodes: %v", err)
	} else {
		cc.logger.Infof("purged %v verification codes", count)
	}

	if count, err := cc.db.PurgeTokens(cc.config.VerificationTokenMaxAge); err != nil {
		cc.logger.Errorf("db.PurgeTokens: %v", err)
	} else {
		cc.logger.Infof("purged %v verification tokens", count)
	}

	if count, err := cc.db.PurgeUsers(cc.config.DisabledUserMaxAge); err != nil {
		cc.logger.Errorf("db.PurgeUsers: %v", err)
	} else {
		cc.logger.Infof("purged %v user records", count)
	}

	controller.WriteJSON(w, http.StatusOK, &cleanupResult{true})
}
