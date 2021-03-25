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

package appsync

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

// HandleSync performs the logic to sync mobile apps.
func (c *Controller) HandleSync() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("appsync.HandleSync")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		ok, err := c.db.TryLock(ctx, appSyncLock, c.config.AppSyncMinPeriod)
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

		apps, err := c.appSyncClient.AppSync(ctx)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// If there are any errors, return them
		if merr := c.syncApps(ctx, apps); merr != nil {
			if errs := merr.WrappedErrors(); len(errs) > 0 {
				logger.Errorw("failed to sync apps", "errors", errs)
				c.h.RenderJSON(w, http.StatusInternalServerError, errs)
				return
			}
		}

		stats.Record(ctx, mSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
