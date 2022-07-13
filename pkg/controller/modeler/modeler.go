// Copyright 2020 the Exposure Notifications Verification Server authors
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

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
	"gonum.org/v1/gonum/mat"

	"github.com/sethvargo/go-limiter"
)

const modelerLock = "modelerLock"

// Controller is a controller for the modeler service.
type Controller struct {
	config  *config.Modeler
	db      *database.Database
	h       *render.Renderer
	limiter limiter.Store
}

// New creates a new modeler controller.
func New(ctx context.Context, config *config.Modeler, db *database.Database, limiter limiter.Store, h *render.Renderer) *Controller {
	return &Controller{
		config:  config,
		db:      db,
		h:       h,
		limiter: limiter,
	}
}

// HandleModel accepts an HTTP trigger and re-generates the models.
func (c *Controller) HandleModel() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("modeler.HandleModel")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, modelerLock, 15*time.Minute)
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

		// Get all realms.
		realms, _, err := c.db.ListRealms(pagination.UnlimitedResults)
		if err != nil {
			logger.Errorw("failed to list realms", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		// Build models for each realm
		var merr *multierror.Error
		for _, realm := range realms {
			if err := c.rebuildAbusePreventionModel(ctx, realm); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to rebuild abuse prevention model for realm %d: %w", realm.ID, err))
			}

			if err := c.rebuildAnomaliesModel(ctx, realm); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to rebuild anomaly model for realm %d: %w", realm.ID, err))
			}
		}

		if errs := merr.WrappedErrors(); len(errs) > 0 {
			logger.Errorw("failed to rebuild models", "errors", errs)
			c.h.RenderJSON(w, http.StatusInternalServerError, errs)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// rebuildAbusePreventionModel builds the abuse prevention model for the realm.
func (c *Controller) rebuildAbusePreventionModel(ctx context.Context, realm *database.Realm) error {
	logger := logging.FromContext(ctx).Named("modeler.rebuildAbusePreventionModel").With("id", realm.ID)

	// Skip if abuse prevention is not enabled on this realm.
	if !realm.AbusePreventionEnabled {
		return nil
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
	beta := mat.NewDense(len(ys), 1, ys)
	gamma := mat.NewDense(degree+1, 1, nil)
	qr := new(mat.QR)
	qr.Factorize(alpha)
	if err := qr.SolveTo(gamma, false, beta); err != nil {
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
	raw := curve(float64(len(ys)))
	logger.Debugw("computed raw next curve", "next", raw)

	// Round the value. There are small floating point number variations between
	// Intel and Arm processors, but they are like 0.0000000004 off. However, this
	// can cause conversion issues, so round.
	nextFloat := math.Round(raw)
	if nextFloat < 0 {
		nextFloat = 0
	}

	// Calculate the predicted next value as a uint.
	next := uint(nextFloat)
	logger.Debugw("computed next float", "next", next)

	// This should really never happen - it means there's been a very sharp
	// decline in the number of codes issued. In that case, we want to revert
	// back to the default minimum.
	if next < c.config.MinValue {
		logger.Debugw("next is less than min, using min", "next", next, "min", c.config.MinValue)
		next = c.config.MinValue
	}

	// Ensure we don't exceed the number at which the math gods get angry.
	if next > c.config.MaxValue {
		logger.Debugw("next is greater than allowed max, using max", "next", next, "max", c.config.MaxValue)
		next = c.config.MaxValue
	}

	logger.Debugw("next value", "value", next)

	// Save the new value back, bypassing any validation.
	realm.AbusePreventionLimit = next
	if err := c.db.SaveRealm(realm, database.System); err != nil {
		return fmt.Errorf("failed to save model: %w, errors: %q", err, realm.ErrorMessages())
	}

	// Calculate effective limit.
	effective := realm.AbusePreventionEffectiveLimit()

	logger.Debugw("next effective limit", "value", effective)

	// Update the limiter to use the new value.
	key, err := realm.QuotaKey(c.config.RateLimit.HMACKey)
	if err != nil {
		return fmt.Errorf("failed to digest realm id: %w", err)
	}
	if err := c.limiter.Set(ctx, key, uint64(effective), 24*time.Hour); err != nil {
		return fmt.Errorf("failed to update limit: %w", err)
	}

	return nil
}

// rebuildAnomaliesModel rebuilds the anomaly detection models.
func (c *Controller) rebuildAnomaliesModel(ctx context.Context, realm *database.Realm) error {
	logger := logging.FromContext(ctx).Named("modeler.rebuildAnamoliesModel").With("id", realm.ID)

	// Fetch the historical statistics.
	realmStats, err := realm.Stats(c.db)
	if err != nil {
		return fmt.Errorf("failed to fetch stats: %w", err)
	}

	// This should never happen because realm.Stats returns zero-padded data, but
	// I prefer verbosity over panics.
	if len(realmStats) < 2 {
		logger.Warnw("skipping, not enough stats points")
		return nil
	}

	// Remove the first entry - that's the most recent date which is likely an
	// incomplete UTC day. Also remove the second entry, since that's the first
	// full UTC day and it's what we'll use to compute the "current" ratio.
	lastCompleteDay, realmStats := realmStats[1], realmStats[2:]

	// Get the last 30 days of stats in which codes have been issued, ignoring any
	// days where zero codes were issued.
	codesRatios := make([]float64, 0, 30)
	for _, stat := range realmStats {
		// Only capture 30 days worth of data.
		if len(codesRatios) == 30 {
			break
		}

		// Only include days where codes were issued. This discards weekends or
		// holidays in which no codes were issued from the model as they are almost
		// always anomalies.
		if stat.CodesIssued == 0 {
			continue
		}

		// Compute the collection of daily ratios of codes claimed vs codes issued.
		codeRatio := float64(stat.CodesClaimed) / float64(stat.CodesIssued)

		// Cap the ratio at 1.0. This can happen because statistics are UTC-bound,
		// and verification codes are valid for 24h. It's possible that a code
		// issued in UTC day 1 isn't claimed until UTC day 2, but that's still
		// within 24h.
		if codeRatio > 1.0 {
			codeRatio = 1.0
		}
		codesRatios = append(codesRatios, codeRatio)
	}

	// Require a minimum number of data points before building a model.
	if got, want := len(codesRatios), 14; got < want {
		logger.Warnw("skipping, not enough data", "points", got)
		return nil
	}

	// Calculate the mean.
	codesMean := mean(codesRatios)

	// Calculate the standard deviation.
	codesStddev := stddev(codesRatios, codesMean)

	// Calculate the means for the the most recent complete day.
	var lastCodes float64
	if lastCompleteDay.CodesIssued == 0 {
		// If no codes were issued on the most recent day, set the ratio to 1. We
		// don't want to trigger the alerting if zero codes were issued.
		lastCodes = 1.0
	} else {
		lastCodes = float64(lastCompleteDay.CodesClaimed) / float64(lastCompleteDay.CodesIssued)
	}
	if lastCodes > 1.0 {
		lastCodes = 1.0
	}

	realm.LastCodesClaimedRatio = lastCodes
	realm.CodesClaimedRatioMean = codesMean
	realm.CodesClaimedRatioStddev = codesStddev
	if err := c.db.SaveRealm(realm, database.System); err != nil {
		return fmt.Errorf("failed to save model: %w, errors: %q", err, realm.ErrorMessages())
	}

	// If the new ratio is anomalous and it's not the e2e realm, emit a metric.
	// The e2e realm has its own existing monitoring for successes.
	if realm.CodesClaimedRatioAnomalous() {
		ctx = observability.WithRealmID(ctx, uint64(realm.ID))
		stats.Record(ctx, mCodesClaimedRatioAnomaly.M(1))
	}

	return nil
}

// mean computes the mean of the slice.
func mean(in []float64) float64 {
	var sum float64
	for _, v := range in {
		sum += v
	}
	return sum / float64(len(in))
}

// stddev calculates the population standard deviation for the slice.
func stddev(in []float64, m float64) float64 {
	var sd float64
	for _, v := range in {
		sd += (v - m) * (v - m)
	}
	return math.Sqrt(sd / float64(len(in)))
}

// vandermonde creates a Vandermonde projection (matrix) of the given degree.
func vandermonde(a []float64, degree int) *mat.Dense {
	x := mat.NewDense(len(a), degree+1, nil)
	for i := range a {
		for j, p := 0, 1.; j <= degree; j, p = j+1, p*a[i] {
			x.Set(i, j, p)
		}
	}
	return x
}
