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

package controller

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func HandleHealthz(db *database.Database, h render.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("controller.HandleHealthz")

		params := r.URL.Query()
		switch service := params.Get("service"); service {
		case "":
			// Do nothing and continue rendering - this is a basic HTTP health check
		case "database":
			if err := db.Ping(ctx); err != nil {
				InternalError(w, r, h, fmt.Errorf("failed to ping db: %w", err))
				return
			}
		case "alerts":
			// TODO(ych): fire a metric and configure an alert
		default:
			logger.Warnw("unknown service", "service", service)
		}

		h.RenderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
