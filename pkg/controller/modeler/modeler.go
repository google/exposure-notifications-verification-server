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

// Package modeler implements periodic statistical calculations.
package modeler

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gonum/matrix/mat64"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-limiter"
	"go.uber.org/zap"
)

// Controller is a controller for the modeler service.
type Controller struct {
	config  *config.Modeler
	db      *database.Database
	h       *render.Renderer
	limiter limiter.Store
	logger  *zap.SugaredLogger
}

// New creates a new modeler controller.
func New(ctx context.Context, config *config.Modeler, db *database.Database, limiter limiter.Store, h *render.Renderer) *Controller {
	logger := logging.FromContext(ctx).Named("modeler")

	return &Controller{
		config:  config,
		db:      db,
		h:       h,
		limiter: limiter,
		logger:  logger,
	}
}

// HandleModel accepts an HTTP trigger and re-generates the models.
func (c *Controller) HandleModel() http.Handler {
	logger := c.logger.Named("HandleModel")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := c.db.ClaimModelerStatus(); err != nil {
			logger.Errorw("failed to claim modeler status", "error", err)
			c.h.JSON500(w, err)
			return
		}

		if err := c.rebuildModels(ctx); err != nil {
			logger.Errorw("failed to build models", "error", err)
			c.h.JSON500(w, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// rebuildModels iterates over all models with abuse prevention enabled,
// calculates the new limits, and updates the new limits.
func (c *Controller) rebuildModels(ctx context.Context) error {
	logger := c.logger.Named("rebuildModels")
	// Get all realm IDs in a single operation so we can iterate realm-by-realm to
	// avoid a full table lock during stats calculation.
	ids, err := c.db.AbusePreventionEnabledRealmIDs()
	if err != nil {
		return fmt.Errorf("failed to fetch ids: %w", err)
	}
	logger.Debugw("building models", "count", len(ids))

	// Process all models.
	var merr *multierror.Error
	for _, id := range ids {
		if err := c.rebuildModel(ctx, id); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("failed to update realm %d: %w", id, err))
		}
	}

	return merr.ErrorOrNil()
}

// rebuildModel rebuilds and updates the model for a single model.
func (c *Controller) rebuildModel(ctx context.Context, id uint64) error {
	logger := c.logger.Named("rebuildModel").With("id", id)

	// Lookup the realm.
	realm, err := c.db.FindRealm(id)
	if err != nil {
		return fmt.Errorf("failed to find realm: %w", err)
	}

	// Get 21 days of historical data for the realm.
	stats, err := realm.HistoricalCodesIssued(c.db, 21)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Require some reasonable number of days of history before attempting to
	// build a model.
	if l := len(stats); l < 14 {
		logger.Warnw("skipping, not enough data", "points", l)
		return nil
	}

	// Exclude the most recent record. Depending on timezones, the "day" might not
	// be over at 00:00 UTC, and we don't want to generate a partial model.
	stats = stats[:len(stats)-1]

	// Reverse the list - it came in reversed because we sorted by date DESC, but
	// the model expects the date to be in ascending order.
	for i, j := 0, len(stats)-1; i < j; i, j = i+1, j-1 {
		stats[i], stats[j] = stats[j], stats[i]
	}

	// Build the list of Xs and Ys.
	xs := make([]float64, len(stats))
	ys := make([]float64, len(stats))
	for i, v := range stats {
		xs[i] = float64(i)
		ys[i] = float64(v)
	}

	// This is probably overkill, but it enables us to pick a different curve in
	// the future, if we want.
	degree := 1
	alpha := vandermonde(xs, degree)
	beta := mat64.NewDense(len(ys), 1, ys)
	gamma := mat64.NewDense(degree+1, 1, nil)
	qr := new(mat64.QR)
	qr.Factorize(alpha)
	if err := gamma.SolveQR(qr, false, beta); err != nil {
		return fmt.Errorf("failed to solve QR: %w", err)
	}

	// Build the curve function.
	m := gamma.RawMatrix()
	curve := func(x float64) float64 {
		var result float64
		for i := len(m.Data) - 1; i >= 0; i-- {
			result += m.Data[i] * math.Pow(x, float64(i))
		}
		return result
	}

	// In the case of a sharp decline, the model might predict a very low value,
	// potentially less than zero. We need to do the negative check against the
	// float value before casting to a uint, or else risk overflowing if this
	// value is negative.
	nextFloat := math.Ceil(curve(float64(len(ys))))
	if nextFloat < 0 {
		nextFloat = 0
	}

	// Calculate the predicted next value as a uint.
	next := uint(nextFloat)

	// This should really never happen - it means there's been a very sharp
	// decline in the number of codes issued. In that case, we want to revert
	// back to the default minimum.
	if next < c.config.MinValue {
		logger.Warnw("next is less than min, using min", "next", next, "min", c.config.MinValue)
		next = c.config.MinValue
	}

	// Ensure we don't exceed the number at which the math gods get angry.
	if next > c.config.MaxValue {
		logger.Warnw("next is greater than allowed max, using max", "next", next, "max", c.config.MaxValue)
		next = c.config.MaxValue
	}

	logger.Debugw("next value", "value", next)

	// Save the new value back, bypassing any validation.
	realm.AbusePreventionLimit = next
	if err := c.db.SaveRealm(realm, database.System); err != nil {
		return fmt.Errorf("failed to save model: %w", err)
	}

	// Calculate effective limit.
	effective := realm.AbusePreventionEffectiveLimit()

	logger.Debugw("next effective limit", "value", effective)

	// Update the limiter to use the new value.
	dig, err := digest.HMACUint64(id, c.config.RateLimit.HMACKey)
	if err != nil {
		return fmt.Errorf("failed to digest realm id: %w", err)
	}
	key := fmt.Sprintf("realm:quota:%s", dig)
	if err := c.limiter.Set(ctx, key, uint64(effective), 24*time.Hour); err != nil {
		return fmt.Errorf("failed to update limit: %w", err)
	}

	return nil
}

// vandermonde creates a Vandermonde projection (matrix) of the given degree.
func vandermonde(a []float64, degree int) *mat64.Dense {
	x := mat64.NewDense(len(a), degree+1, nil)
	for i := range a {
		for j, p := 0, 1.; j <= degree; j, p = j+1, p*a[i] {
			x.Set(i, j, p)
		}
	}
	return x
}
