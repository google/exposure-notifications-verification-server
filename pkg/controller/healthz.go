package controller

import (
	"context"
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
