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

package cleanup

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/tag"
)

func (c *Controller) HandleCleanup() http.Handler {
	type CleanupResult struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("cleanup.HandleCleanup")

		var result, item tag.Mutator

		ok, err := c.db.TryLock(ctx, cleanupName, c.config.CleanupMinPeriod)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &CleanupResult{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, &CleanupResult{
				OK:     false,
				Errors: []string{"too early"},
			})
			return
		}

		// Construct a multi-error. If one of the purges fails, we still want to
		// attempt the other purges.
		var merr *multierror.Error

		// API keys
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "API_KEYS")
			if count, err := c.db.PurgeAuthorizedApps(c.config.AuthorizedAppMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge authorized apps: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged authorized apps", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Verification codes - purge codes from database entirely.
		// Their code/long_code hmac values will have been set to "".
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_CODE")
			if count, err := c.db.PurgeVerificationCodes(c.config.VerificationCodeStatusMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge verification codes: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged verification codes", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Verification codes - recycle codes. Zero out the code/long_code values
		// so status can be reported, but codes couldn't be recalculated or checked.
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_CODE_RECYCLE")
			if count, err := c.db.RecycleVerificationCodes(c.config.VerificationCodeMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge verification codes: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("recycled verification codes", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Verification tokens
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_TOKEN")
			if count, err := c.db.PurgeTokens(c.config.VerificationTokenMaxAge); err != nil {
				result = enobs.ResultError("FAILED")
				merr = multierror.Append(merr, fmt.Errorf("failed to purge tokens: %w", err))
			} else {
				logger.Infow("purged verification tokens", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Mobile apps
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "MOBILE_APP")
			if count, err := c.db.PurgeMobileApps(c.config.MobileAppMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge mobile apps: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged mobile apps", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Audit entries
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "AUDIT_ENTRY")
			if count, err := c.db.PurgeAuditEntries(c.config.AuditEntryMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge audit entries: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged audit entries", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Users
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "USER")
			if count, err := c.db.PurgeUsers(c.config.UserPurgeMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge users: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged user entries", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Token signing keys
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "TOKEN_SIGNING_KEY")
			if count, err := c.db.PurgeTokenSigningKeys(ctx, c.signingTokenKeyManager, c.config.SigningTokenKeyMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge token signing keys: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged token signing keys", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Verification signing key references
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "VERIFICATION_SIGNING_KEY")
			if count, err := c.db.PurgeSigningKeys(c.config.VerificationSigningKeyMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge verification signing keys: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged verification signing keys", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Key server stats
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "KEY_SERVER_STATS")
			if count, err := c.db.DeleteOldKeyServerStatsDays(c.config.KeyServerStatsMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge key-server stats: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged key-server stats", "count", count)
				result = enobs.ResultOK
			}
		}()

		// Realm chaff events
		func() {
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)
			item = tag.Upsert(itemTagKey, "REALM_CHAFF_EVENT")
			if count, err := c.db.PurgeRealmChaffEvents(c.config.RealmChaffEventMaxAge); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to purge realm chaff events: %w", err))
				result = enobs.ResultError("FAILED")
			} else {
				logger.Infow("purged realm chaff events", "count", count)
				result = enobs.ResultOK
			}
		}()

		// If there are any errors, return them
		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to cleanup", "errors", errs)
				c.h.RenderJSON(w, http.StatusInternalServerError, &CleanupResult{
					OK:     false,
					Errors: project.ErrorsToStrings(errs),
				})
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK, &CleanupResult{
			OK: true,
		})
	})
}
