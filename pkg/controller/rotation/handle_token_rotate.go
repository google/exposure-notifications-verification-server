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

package rotation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// HandleRotate handles key rotation.
func (c *Controller) HandleRotate() http.Handler {
	type Result struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("rotation.HandleRotate")

		var merr *multierror.Error

		ok, err := c.db.TryLock(ctx, tokenRotationLock, c.config.MinTTL)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, &Result{
				OK:     false,
				Errors: []string{"too early"},
			})
			return
		}

		// Token signing keys
		func() {
			item := tag.Upsert(itemTagKey, "TOKEN_SIGNING_KEYS")
			result := enobs.ResultOK
			defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &item)

			existing, err := c.db.ActiveTokenSigningKey()
			if err != nil && !database.IsNotFound(err) {
				merr = multierror.Append(merr, fmt.Errorf("failed to lookup existing signing key: %w", err))
				result = enobs.ResultError("FAILED")
				return
			}
			if existing != nil {
				if age, max := time.Now().UTC().Sub(existing.CreatedAt), c.config.TokenSigningKeyMaxAge; age < max {
					logger.Debugw("token signing key does not require rotation", "age", age, "max", max)
					return
				}
			}

			key, err := c.db.RotateTokenSigningKey(ctx, c.keyManager, c.config.TokenSigning.TokenSigningKey, RotationActor)
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to rotate token signing key: %w", err))
				result = enobs.ResultError("FAILED")
				return
			}

			logger.Infow("rotated token signing key", "new", key)
		}()

		// If there are any errors, return them
		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to rotate", "errors", errs)
				c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
					OK:     false,
					Errors: project.ErrorsToStrings(errs),
				})
				return
			}
		}

		stats.Record(ctx, mTokenSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}
