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
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	v1 "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/certapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
)

const (
	statsPullerLock = "statsPullerLock"
)

// HandlePullStats pulls key-server statistics.
func (c *Controller) HandlePullStats() http.Handler {
	type Result struct {
		OK     bool     `json:"ok"`
		Errors []string `json:"errors,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("statspuller.HandlePullStats")

		ok, err := c.db.TryLock(ctx, statsPullerLock, c.config.StatsPullerMinPeriod)
		if err != nil {
			logger.Errorw("failed to acquite lock", "error", err)
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

		// Get all of the realms with stats configured
		statsConfigs, err := c.db.ListKeyServerStats()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		for _, realmStat := range statsConfigs {
			realmID := realmStat.RealmID

			var err error
			client := c.defaultKeyServerClient
			if realmStat.KeyServerURLOverride != "" {
				client, err = clients.NewKeyServerClient(
					realmStat.KeyServerURLOverride,
					clients.WithTimeout(c.config.DownloadTimeout),
					clients.WithMaxBodySize(c.config.FileSizeLimitBytes))
				if err != nil {
					logger.Errorw("failed to create key server client", "error", err)
					continue
				}
			}

			s, err := certapi.GetSignerForRealm(ctx, realmID, c.config.CertificateSigning, c.signerCache, c.db, c.kms)
			if err != nil {
				logger.Errorw("failed to retrieve signer for realm", "realmID", realmID, "error", err)
				continue
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
				logger.Errorw("failed to stat-pull token", "error", err)
				continue
			}

			resp, err := client.Stats(ctx, &v1.StatsRequest{}, signedJWT)
			if err != nil {
				logger.Errorw("failed make stats call", "error", err)
				continue
			}

			for _, d := range resp.Days {
				if d == nil {
					continue
				}
				day := database.MakeKeyServerStatsDay(realmID, d)
				if err = c.db.SaveKeyServerStatsDay(day); err != nil {
					logger.Errorw("failed saving stats day", "error", err)
				}
			}
		}

		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}
