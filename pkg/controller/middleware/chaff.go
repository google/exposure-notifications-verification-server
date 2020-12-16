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

package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
)

const (
	// ChaffHeader is the chaff header key.
	ChaffHeader = "X-Chaff"

	// ChaffDailyKey is the key to check if the chaff should be counted toward
	// daily stats.
	ChaffDailyKey = "daily"
)

// ProcessChaff injects the chaff processing middleware. If chaff requests send
// a value of "daily" (case-insensitive), they will be counted toward the
// realm's total active users and return a chaff response. Any other values will
// only return a chaff response.
//
// This must come after RequireAPIKey.
func ProcessChaff(db *database.Database, t *chaff.Tracker) mux.MiddlewareFunc {
	detector := chaff.HeaderDetector(ChaffHeader)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			now := time.Now().UTC()

			logger := logging.FromContext(ctx).Named("middleware.ProcessChaff")

			chaffValue := project.TrimSpace(strings.ToLower(r.Header.Get(ChaffHeader)))
			if chaffValue == ChaffDailyKey {
				currentRealm := controller.RealmFromContext(ctx)
				if currentRealm == nil {
					logger.Error("missing current realm in context")
				} else if currentRealm.DailyActiveUsersEnabled {
					// Increment DAU asynchronously and out-of-band for the request. These
					// statistics are best-effort and we don't want to block or delay
					// rendering to populate stats.
					go func() {
						if err := currentRealm.IncrementDailyActiveUsers(db, now); err != nil {
							logger.Errorw("failed to increment daily active stats",
								"realm", currentRealm.ID,
								"error", err)
						}
					}()
				}
			}

			t.HandleTrack(detector, next).ServeHTTP(w, r)
		})
	}
}
