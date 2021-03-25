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

package statspuller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	v1 "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/hashicorp/go-multierror"
	"github.com/sethvargo/go-retry"
	"go.opencensus.io/stats"
	"golang.org/x/sync/semaphore"
)

const (
	statsPullerLock = "statsPullerLock"
)

// HandlePullStats pulls key-server statistics.
func (c *Controller) HandlePullStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("statspuller.HandlePullStats")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, statsPullerLock, c.config.StatsPullerMinPeriod)
		if err != nil {
			logger.Errorw("failed to acquite lock", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			logger.Debugw("skipping (too early)")
			c.h.RenderJSON(w, http.StatusOK, fmt.Errorf("too early"))
			return
		}

		// Get all of the realms with stats configured
		statsConfigs, err := c.db.ListKeyServerStats()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		var merr *multierror.Error
		var merrLock sync.Mutex
		sem := semaphore.NewWeighted(c.config.MaxWorkers)
		var wg sync.WaitGroup
		for _, realmStat := range statsConfigs {
			if err := sem.Acquire(ctx, 1); err != nil {
				controller.InternalError(w, r, c.h, fmt.Errorf("failed to acquire semaphore: %w", err))
				return
			}

			wg.Add(1)
			go func(ctx context.Context, realmStat *database.KeyServerStats) {
				defer sem.Release(1)
				defer wg.Done()
				if err := c.pullOneStat(ctx, realmStat); err != nil {
					merrLock.Lock()
					defer merrLock.Unlock()
					merr = multierror.Append(merr, fmt.Errorf("failed to pull stats for realm %d: %w", realmStat.RealmID, err))
				}
			}(ctx, realmStat)
		}
		wg.Wait()

		if errs := merr.WrappedErrors(); len(errs) > 0 {
			logger.Errorw("failed to pull stats", "errors", errs)
			c.h.RenderJSON(w, http.StatusInternalServerError, errs)
			return
		}

		stats.Record(ctx, mSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

func (c *Controller) pullOneStat(ctx context.Context, realmStat *database.KeyServerStats) error {
	realmID := realmStat.RealmID

	client := c.defaultKeyServerClient
	if realmStat.KeyServerURLOverride != "" {
		var err error
		client, err = clients.NewKeyServerClient(
			realmStat.KeyServerURLOverride,
			clients.WithTimeout(c.config.DownloadTimeout),
			clients.WithMaxBodySize(c.config.FileSizeLimitBytes))
		if err != nil {
			return fmt.Errorf("failed to create key server client: %w", err)
		}
	}

	s, err := certapi.GetSignerForRealm(ctx, realmID, c.config.CertificateSigning, c.signerCache, c.db, c.kms)
	if err != nil {
		return fmt.Errorf("failed to retrieve signer for realm %d: %w", realmID, err)
	}

	audience := c.config.KeyServerStatsAudience
	if realmStat.KeyServerAudienceOverride != "" {
		audience = realmStat.KeyServerAudienceOverride
	}

	now := time.Now().UTC()
	claims := &jwt.StandardClaims{
		Audience:  audience,
		ExpiresAt: now.Add(5 * time.Minute).UTC().Unix(),
		IssuedAt:  now.Unix(),
		Issuer:    s.Issuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.KeyID

	signedJWT, err := jwthelper.SignJWT(token, s.Signer)
	if err != nil {
		return fmt.Errorf("failed to stat-pull token: %w", err)
	}

	// Attempt to download the stats with retries. We intentionally re-use the
	// same JWT because it's valid for 5min and don't want the overhead of
	// reconstructing and signing it.
	var resp *v1.StatsResponse
	b, _ := retry.NewConstant(500 * time.Millisecond)
	b = retry.WithMaxRetries(3, b)
	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		var err error
		resp, err = client.Stats(ctx, &v1.StatsRequest{}, signedJWT)
		if err != nil {
			return retry.RetryableError(fmt.Errorf("failed to make stats call: %w", err))
		}
		return nil
	}); err != nil {
		return errors.Unwrap(err)
	}

	for _, d := range resp.Days {
		if d == nil {
			continue
		}
		day := database.MakeKeyServerStatsDay(realmID, d)
		if err = c.db.SaveKeyServerStatsDay(day); err != nil {
			return fmt.Errorf("failed to save stats day: %w", err)
		}
	}

	return nil
}
