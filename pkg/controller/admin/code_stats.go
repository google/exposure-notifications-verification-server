package admin

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

// HandleCodeStats returns the code issue / claimed
// rates across all realms.
func (c *Controller) HandleCodeStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		stats, err := c.db.AllRealmCodeStatsCached(ctx, c.cacher, int(c.config.MinRealmsForSystemStatistics))
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, stats)
	})
}
