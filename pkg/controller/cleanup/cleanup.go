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
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
)

// Controller is a controller for the cleanup service.
type Controller struct {
	config *config.CleanupConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// New creates a new cleanup controller.
func New(ctx context.Context, config *config.CleanupConfig, db *database.Database, h *render.Renderer) (*Controller, error) {
	logger := logging.FromContext(ctx).Named("cleanup")

	return &Controller{
		config: config,
		db:     db,
		h:      h,
		logger: logger,
	}, nil
}

func (c *Controller) shouldCleanup(ctx context.Context) error {
	cStat, err := c.db.CreateCleanup(database.CleanupName)
	if err != nil {
		return fmt.Errorf("failed to create cleanup: %w", err)
	}

	if cStat.NotBefore.After(time.Now().UTC()) {
		return fmt.Errorf("skipping cleanup, no cleanup before %v", cStat.NotBefore)
	}

	// Attempt to advance the generation.
	stats.Record(ctx, mClaimAttempts.M(1))
	if _, err = c.db.ClaimCleanup(cStat, c.config.CleanupPeriod); err != nil {
		stats.Record(ctx, mClaimErrors.M(1))
		return fmt.Errorf("failed to claim cleanup: %w", err)
	}
	return nil
}

func (c *Controller) HandleCleanup() http.Handler {
	type CleanupResult struct {
		OK     bool    `json:"ok"`
		Errors []error `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		if err := c.shouldCleanup(ctx); err != nil {
			c.logger.Errorw("failed to run shouldCleanup", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &CleanupResult{
				OK:     false,
				Errors: []error{err},
			})
			return
		}

		// Construct a multi-error. If one of the purges fails, we still want to
		// attempt the other purges.
		var merr *multierror.Error

		// Verification codes
		stats.Record(ctx, mPurgeVerificationCodesAttempts.M(1))
		if count, err := c.db.PurgeVerificationCodes(c.config.VerificationCodeMaxAge); err != nil {
			stats.Record(ctx, mPurgeVerificationCodesErrors.M(1))
			merr = multierror.Append(merr, fmt.Errorf("failed to purge verification codes: %w", err))
		} else {
			stats.Record(ctx, mPurgeVerificationCodesPurged.M(count))
			c.logger.Infow("purged verification codes", "count", count)
		}

		// Verification tokens
		stats.Record(ctx, mPurgeVerificationTokensAttempts.M(1))
		if count, err := c.db.PurgeTokens(c.config.VerificationTokenMaxAge); err != nil {
			stats.Record(ctx, mPurgeVerificationTokensErrors.M(1))
			merr = multierror.Append(merr, fmt.Errorf("failed to purge tokens: %w", err))
		} else {
			stats.Record(ctx, mPurgeVerificationTokensPurged.M(count))
			c.logger.Infow("purged verification tokens", "count", count)
		}

		// Mobile apps
		stats.Record(ctx, mPurgeMobileAppsAttempts.M(1))
		if count, err := c.db.PurgeMobileApps(c.config.MobileAppMaxAge); err != nil {
			stats.Record(ctx, mPurgeMobileAppsErrors.M(1))
			merr = multierror.Append(merr, fmt.Errorf("failed to purge mobile apps: %w", err))
		} else {
			stats.Record(ctx, mPurgeMobileAppsPurged.M(count))
			c.logger.Infow("purged mobile apps", "count", count)
		}

		// Audit entries
		stats.Record(ctx, mPurgeAuditEntriesAttempts.M(1))
		if count, err := c.db.PurgeAuditEntries(c.config.AuditEntryMaxAge); err != nil {
			stats.Record(ctx, mPurgeAuditEntriesErrors.M(1))
			merr = multierror.Append(merr, fmt.Errorf("failed to purge audit entries: %w", err))
		} else {
			stats.Record(ctx, mPurgeAuditEntriesPurged.M(count))
			c.logger.Infow("purged audit entries", "count", count)
		}

		// If there are any errors, return them
		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				c.logger.Errorw("failed to cleanup", "errors", errs)
				c.h.RenderJSON(w, http.StatusInternalServerError, &CleanupResult{
					OK:     false,
					Errors: errs,
				})
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK, &CleanupResult{
			OK: true,
		})
	})
}
