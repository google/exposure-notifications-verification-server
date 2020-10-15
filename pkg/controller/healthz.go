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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"golang.org/x/time/rate"
)

var (
	rl = rate.NewLimiter(rate.Every(time.Minute), 1)
)

func HandleHealthz(hctx context.Context, cfg *database.Config, h *render.Renderer) http.Handler {
	logger := logging.FromContext(hctx).Named("healthz")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		params := r.URL.Query()
		if s := params.Get("service"); s == "database" {
			if cfg == nil {
				InternalError(w, r, h, fmt.Errorf("database not configured for health check"))
				return
			}

			if rl.Allow() {
				db, err := cfg.Load(ctx)
				if err != nil {
					logger.Errorw("config db", "error", err)
					InternalError(w, r, h, err)
					return
				}

				if err := db.Open(ctx); err != nil {
					logger.Errorw("connect db", "error", err)
					InternalError(w, r, h, err)
					return
				}
				defer db.Close()

				if err := db.Ping(ctx); err != nil {
					logger.Errorw("ping db", "error", err)
					InternalError(w, r, h, err)
					return
				}
			}
		}

		h.RenderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}
