package controller

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func HandleHealthz(h *render.Renderer, cfg *database.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("healthz")
		params := r.URL.Query()
		if s := params.Get("service"); s == "database" {
			db, err := cfg.Open(ctx)
			if err != nil {
				logger.Errorw("database error", err)
				h.JSON500(w, err)
				return
			}
			defer db.Close()

			if err := db.Ping(); err != nil {
				logger.Errorw("database error", err)
				h.JSON500(w, err)
				return
			}
		}

		h.RenderJSON(w, http.StatusOK, nil)
	})
}
