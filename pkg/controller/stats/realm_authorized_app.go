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

// Package stats produces statistics.
package stats

import (
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

// HandleRealmAuthorizedAppStats renders statistics for an authorized app in the
// current realm.
func (c *Controller) HandleRealmAuthorizedAppStats(typ Type) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		currentRealm, ok := authorizeFromContext(ctx, rbac.StatsRead, rbac.APIKeyRead)
		if !ok {
			controller.Unauthorized(w, r, c.h)
			return
		}

		authorizedApp, err := currentRealm.FindAuthorizedApp(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		stats, err := authorizedApp.StatsCached(ctx, c.db, c.cacher)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		switch typ {
		case TypeCSV:
			filename := notFilenameRe.ReplaceAllString(strings.ToLower(authorizedApp.Name), "-")
			c.h.RenderCSV(w, http.StatusOK, csvFilename(filename), stats)
			return
		case TypeJSON:
			c.h.RenderJSON(w, http.StatusOK, stats)
			return
		default:
			controller.NotFound(w, r, c.h)
			return
		}
	})
}
