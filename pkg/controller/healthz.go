package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"golang.org/x/time/rate"
)

var (
	rl = rate.NewLimiter(rate.Every(time.Minute), 1)

	mHealthzBuildInfo = stats.Int64("build_info", "Current build information", "1")
)

func HandleHealthz(ctx context.Context, cfg *database.Config, h *render.Renderer) http.Handler {
	logger := logging.FromContext(ctx).Named("healthz")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		// Emit the buildinfo metric
		stats.Record(ctx, mHealthzBuildInfo.M(1))

		params := r.URL.Query()
		if params.Get("service") == "database" {
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
