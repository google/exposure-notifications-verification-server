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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/hashicorp/go-multierror"
)

// HandleVerificationRotate handles verification certificate key rotation.
func (c *Controller) HandleVerificationRotate() http.Handler {
	type Result struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("rotation.HandleVerificationRotate")
		ctx = logging.WithLogger(ctx, logger)

		ok, err := c.db.TryLock(ctx, verificationRotationLock, c.config.MinTTL)
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

		var merr *multierror.Error

		realms, _, err := c.db.ListRealms(pagination.UnlimitedResults, database.WithRealmAutoKeyRotationEnabled(true))
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("unable to list realms to rotate signing keys: %w", err))
		}

		if len(realms) > 0 {
			if err := c.createNewKeys(ctx, realms); err != nil {
				merr = multierror.Append(merr, err)
			}

			if err := c.activateKeys(ctx, realms); err != nil {
				merr = multierror.Append(merr, err)
			}
		}

		// If there are any errors, return them
		if merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to rotate verification keys", "errors", errs)
				c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
					OK:     false,
					Errors: project.ErrorsToStrings(errs),
				})
				return
			}
		}

		stats.Record(ctx, mVerificationSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}

func (c *Controller) createNewKeys(ctx context.Context, realms []*database.Realm) error {
	logger := logging.FromContext(ctx)
	now := time.Now().UTC()
	var merr *multierror.Error

	for _, realm := range realms {
		keys, err := realm.ListSigningKeys(c.db)
		if err != nil {
			logger.Errorw("unable to list signing keys for realm", "realm", realm.ID, "error", err)
			merr = multierror.Append(merr, fmt.Errorf("unable to list signing keys for realm %d: %w", realm.ID, err))
			continue
		}
		// if there isn't a key, or the most recently created key is "too old" - create a new key.
		if len(keys) == 0 || (keys[0].Active && keys[0].CreatedAt.Add(c.config.VerificationSigningKeyMaxAge).Before(now)) {
			if _, err := realm.CreateSigningKeyVersion(ctx, c.db, RotationActor); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("unable to create signing key for realm %d: %w", realm.ID, err))
				continue
			}
			logger.Infow("created new verification signing key", "realm", realm.ID)
		}
	}

	return nil
}

func (c *Controller) activateKeys(ctx context.Context, realms []*database.Realm) error {
	logger := logging.FromContext(ctx)
	now := time.Now().UTC()
	var merr *multierror.Error

	for _, realm := range realms {
		keys, err := realm.ListSigningKeys(c.db)
		if err != nil {
			logger.Errorw("unable to list signing keys for realm", "realm", realm.ID, "error", err)
			merr = multierror.Append(merr, fmt.Errorf("unable to list signing keys for realm %d: %w", realm.ID, err))
			continue
		}

		if len(keys) == 0 {
			continue
		}

		// If most recent key isn't active - see if it is old enough to become active
		if !keys[0].Active && keys[0].CreatedAt.Add(c.config.VerificationActivationDelay).Before(now) {
			if _, err := realm.SetActiveSigningKey(c.db, keys[0].ID, RotationActor); err != nil {
				logger.Errorw("unable to set active signing key for realm", "realm", realm.ID, "error", err)
				merr = multierror.Append(merr, err)
				continue
			}

			logger.Infow("activated new realm signing key", "realm", realm.ID, "kid", keys[0].GetKID())
		}

		// Destroy any keys that are eligible for destruction.
		if len(keys) > 1 {
			for i := 1; i < len(keys); i++ {
				if !keys[i].Active && keys[i].UpdatedAt.Add(c.config.VerificationActivationDelay).Before(now) {
					if err := realm.DestroySigningKeyVersion(ctx, c.db, keys[i].ID, RotationActor); err != nil {
						logger.Errorw("failed to destroy signing key", "realm", realm.ID, "error", err)
						merr = multierror.Append(merr, err)
						continue
					}
					logger.Infow("destroyed signing key", "realm", realm.ID, "kid", keys[i].GetKID())
				}
			}
		}
	}

	return nil
}
