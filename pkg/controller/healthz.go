package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"golang.org/x/time/rate"
)

var (
	rl = rate.NewLimiter(rate.Every(time.Minute), 1)
)

func HandleHealthz(hctx context.Context, h *render.Renderer, cfg *database.Config) http.Handler {
	logger := logging.FromContext(hctx).Named("healthz")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		params := r.URL.Query()
		if s := params.Get("service"); s == "database" {
			if rl.Allow() {
				db, err := cfg.Open(ctx)
				if err != nil {
					logger.Errorw("database error", "error", err)
					h.JSON500(w, h.InternalError)
					return
				}
				defer db.Close()

				if err := db.Ping(ctx); err != nil {
					logger.Errorw("database error", "error", err)
					h.JSON500(w, h.InternalError)
					return
				}
			}
		}

		h.RenderJSON(w, http.StatusOK, nil)
	})
}
