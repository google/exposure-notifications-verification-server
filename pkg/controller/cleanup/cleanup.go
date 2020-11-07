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

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// Controller is a controller for the cleanup service.
type Controller struct {
	config *config.CleanupConfig
	db     *database.Database
	h      *render.Renderer
}

// New creates a new cleanup controller.
func New(ctx context.Context, config *config.CleanupConfig, db *database.Database, h *render.Renderer) (*Controller, error) {
	return &Controller{
		config: config,
		db:     db,
		h:      h,
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
	if _, err = c.db.ClaimCleanup(cStat, c.config.CleanupPeriod); err != nil {
		stats.RecordWithTags(ctx, []tag.Mutator{observability.ResultNotOK()}, mClaimRequests.M(1))
		return fmt.Errorf("failed to claim cleanup: %w", err)
	}
	stats.RecordWithTags(ctx, []tag.Mutator{observability.ResultOK()}, mClaimRequests.M(1))
	return nil
}

func (c *Controller) HandleCleanup() http.Handler {
	type CleanupResult struct {
		OK     bool    `json:"ok"`
		Errors []error `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		logger := logging.FromContext(ctx).Named("cleanup.HandleCleanup")

		var result, item tag.Mutator

		if err := c.shouldCleanup(ctx); err != nil {
			logger.Errorw("failed to run shouldCleanup", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &CleanupResult{
				OK:     false,
				Errors: []error{err},
			})
			return
		}

		// Construct a multi-error. If one of the purges fails, we still want to
		// attempt the other purges.
		var merr *multierror.Error

		// API keys
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "API_KEYS")
			if count, err := c.db.PurgeAuthorizedApps(c.config.AuthorizedAppMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge authorized apps: %w", err))
				result = observability.ResultError("FAILED")
			} else {
				logger.Infow("purged authorized apps", "count", count)
				result = observability.ResultOK()
			}
		}()

		// Verification codes - purge codes from database entirerly.
		// Their code/long_code hmac values will have been set to "".
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_CODE")
			if count, err := c.db.PurgeVerificationCodes(c.config.VerificationCodeMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge verification codes: %w", err))
				result = observability.ResultError("FAILED")
			} else {
				logger.Infow("purged verification codes", "count", count)
				result = observability.ResultOK()
			}
		}()

		// Verification codes - recycle codes. Zero out the code/long_code values
		// so status can be reported, but codes couldn't be recalculated or checked.
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_CODE_RECYCLE")
			if count, err := c.db.RecycleVerificationCodes(c.config.VerificationCodeStatusMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge verification codes: %w", err))
				result = observability.ResultError("FAILED")
			} else {
				logger.Infow("recycled verification codes", "count", count)
				result = observability.ResultOK()
			}
		}()

		// Verification tokens
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_TOKEN")
			if count, err := c.db.PurgeTokens(c.config.VerificationTokenMaxAge); err != nil {
				result = observability.ResultError("FAILED")
				merr = multierror.Append(merr, fmt.Errorf("failed to purge tokens: %w", err))
			} else {
				logger.Infow("purged verification tokens", "count", count)
				result = observability.ResultOK()
			}
		}()

		// Mobile apps
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "MOBILE_APP")
			if count, err := c.db.PurgeMobileApps(c.config.MobileAppMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge mobile apps: %w", err))
				result = observability.ResultError("FAILED")
			} else {
				logger.Infow("purged mobile apps", "count", count)
				result = observability.ResultOK()
			}
		}()

		// Audit entries
		func() {
			defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "AUDIT_ENTRY")
			if count, err := c.db.PurgeAuditEntries(c.config.AuditEntryMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge audit entries: %w", err))
				result = observability.ResultError("FAILED")
			} else {
				logger.Infow("purged audit entries", "count", count)
				result = observability.ResultOK()
			}
		}()

		// If there are any errors, return them
		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to cleanup", "errors", errs)
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
