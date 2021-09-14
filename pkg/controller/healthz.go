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

package controller

import (
	"context"
	"database/sql/driver"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/sethvargo/go-retry"
)

func HandleHealthz(pinger driver.Pinger, h *render.Renderer, isMaintenanceMode bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("controller.HandleHealthz")

		params := r.URL.Query()
		switch service := params.Get("service"); service {
		case "":
			// Do nothing and continue rendering - this is a basic HTTP health check
		case "database":
			// Attempt to ping for up to 1s.
			b, err := retry.NewConstant(200 * time.Millisecond)
			if err != nil {
				InternalError(w, r, h, fmt.Errorf("failed to create backoff: %w", err))
				return
			}
			if err := retry.Do(ctx, retry.WithMaxRetries(5, b), func(ctx context.Context) error {
				if err := pinger.Ping(ctx); err != nil {
					return retry.RetryableError(err)
				}
				return nil
			}); err != nil {
				InternalError(w, r, h, fmt.Errorf("failed to ping db: %w", err))
				return
			}
		default:
			logger.Warnw("unknown service", "service", service)
		}

		h.RenderJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "maintenance_mode": isMaintenanceMode})
	})
}
