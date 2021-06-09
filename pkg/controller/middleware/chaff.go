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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/mikehelmick/go-chaff"
)

// ChaffHeader is the chaff header key.
const ChaffHeader = "X-Chaff"

// ChaffHeaderDetector returns a chaff header detector.
func ChaffHeaderDetector() chaff.Detector {
	return chaff.HeaderDetector(ChaffHeader)
}

// localChaffCache is a local, in-memory cache of realms that have incremented
// chaff on the given UTC day. Values are stored as "<utc_day>:<realm_id>" and
// the cache purges after 48 hours. This exists to alleviate pressure on the
// database.
//
// cache.New only returns an error if the duration is negative, so we ignore the
// error here.
var localChaffCache, _ = cache.New(48 * time.Hour)

// ProcessChaff injects the chaff processing middleware. If chaff requests send
// a value of "daily" (case-insensitive), they will be counted toward the
// realm's total active users and return a chaff response. Any other values will
// only return a chaff response.
//
// This must come after RequireAPIKey.
func ProcessChaff(db *database.Database, t *chaff.Tracker, det chaff.Detector) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if v := r.Header.Get(ChaffHeader); v != "" {
				now := timeutils.UTCMidnight(time.Now().UTC())
				go recordChaffEvent(ctx, now, controller.RealmFromContext(ctx), db)
			}

			// Process normal chaff tracking
			t.HandleTrack(det, next).ServeHTTP(w, r)
		})
	}
}

// recordChaff event annotates that the realm received a chaff request on the
// provided date.
func recordChaffEvent(ctx context.Context, t time.Time, realm *database.Realm, db *database.Database) {
	if realm == nil || db == nil {
		return
	}

	key := fmt.Sprintf("%s:%d", t.Format("2006-01-02"), realm.ID)
	if _, err := localChaffCache.WriteThruLookup(key, func() (interface{}, error) {
		if err := realm.RecordChaffEvent(db, t); err != nil {
			return nil, err
		}
		return &struct{}{}, nil
	}); err != nil {
		logger := logging.FromContext(ctx).Named("chaff.recordChaffEvent")
		logger.Errorw("failed to record chaff event", "realm", realm.ID, "error", err)
	}
}
