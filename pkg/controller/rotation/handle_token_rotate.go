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

package rotation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

// HandleRotateTokenSigningKey handles key rotation.
func (c *Controller) HandleRotateTokenSigningKey() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("rotation.HandleRotate")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, tokenRotationLock, c.config.MinTTL)
		if err != nil {
			logger.Errorw("failed to acquire lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
			return
		}

		// If there are any errors, return them
		if err := c.RotateTokenSigningKey(ctx); err != nil {
			logger.Errorw("failed to rotate", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mTokenSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// RotateTokenSigningKey rotates the signing key. It does not acquire a lock.
func (c *Controller) RotateTokenSigningKey(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("rotation.RotateTokenSigningKey")

	var merr *multierror.Error

	// Token signing keys
	func() {
		logger.Debugw("rotating token signing key")
		defer logger.Debugw("finished rotating token signing key")

		existing, err := c.db.ActiveTokenSigningKey()
		if err != nil && !database.IsNotFound(err) {
			merr = multierror.Append(merr, fmt.Errorf("failed to lookup existing signing key: %w", err))
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
			return
		}
		logger.Infow("rotated token signing key", "new", key)
	}()

	return merr.ErrorOrNil()
}
