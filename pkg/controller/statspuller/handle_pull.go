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

		ok, err := c.db.TryLock(ctx, statsPullerLock, c.config.StatsPullerMinPeriod)
		if err != nil {
			c.h.RenderJSON(w, http.StatusInternalServerError, &Result{
				OK:     false,
				Errors: []string{err.Error()},
			})
			return
		}
		if !ok {
			c.h.RenderJSON(w, http.StatusOK, &Result{
				OK:     false,
				Errors: []string{"too early"},
			})
			return
		}

		logger := logging.FromContext(ctx).Named("rotation.HandlePullStats")
		logger.Debug("no-op stats pull") // TODO(whaught): remove this and put in logic

		// Get all of the realms with stats configured
		statsConfigs, err := c.db.ListKeyServerStats()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		for _, realmStat := range statsConfigs {
			audience := c.config.CertificateSigning.CertificateAudience
			if realmStat.KeyServerAudienceOverride != "" {
				audience = realmStat.KeyServerAudienceOverride
			}

			var err error
			client := c.defaultKeyServerClient
			if realmStat.KeyServerURLOverride != "" {
				client, err = clients.NewKeyServerClient(
					realmStat.KeyServerURLOverride,
					c.config.KeyServerAPIKey,
					clients.WithTimeout(c.config.Timeout),
					clients.WithMaxBodySize(c.config.FileSizeLimitBytes))
				if err != nil {
					logger.Errorw("failed to create key server client", "error", err)
					continue
				}
			}

			s, err := c.getSignerForRealm(ctx, realmStat.RealmID)
			if err != nil {
				logger.Errorw("failed to retrieve signer for realm", "realmID", realmStat.RealmID)
				continue
			}

			now := time.Now().UTC()
			claims := &jwt.StandardClaims{
				Audience:  audience,
				ExpiresAt: now.Add(time.Minute).Unix(),
				IssuedAt:  now.Unix(),
				Issuer:    s.Issuer,
			}
			token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
			token.Header["kid"] = s.KeyID

			jwtString, err := token.SignedString(c.config.CertificateSigning.CertificateSigningKey)
			if err != nil {
				logger.Errorw("failed to sign JWT", "error", err)
				continue
			}

			_, err = client.Stats(ctx, &v1.StatsRequest{}, jwtString)
			if err != nil {
				logger.Errorw("failed make stats call", "error", err)
			}

			// TODO(whaught): interpret the response and store
		}

		c.h.RenderJSON(w, http.StatusOK, &Result{
			OK: true,
		})
	})
}
