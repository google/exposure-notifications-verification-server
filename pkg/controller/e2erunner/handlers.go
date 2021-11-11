// Copyright 2021 the Exposure Notifications Verification Server authors
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

package e2erunner

import (
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/clients"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/stats"
)

// HandleDefault handles the default end-to-end scenario.
func (c *Controller) HandleDefault() http.Handler {
	cfg := *c.config
	cfg.DoRevise = false
	cfg.DoUserReport = false
	return c.handleEndToEnd(&cfg, mDefaultSuccess)
}

// HandleRevise runs the end-to-end runner with revision tokens.
func (c *Controller) HandleRevise() http.Handler {
	cfg := *c.config
	cfg.DoRevise = true
	cfg.DoUserReport = false
	return c.handleEndToEnd(&cfg, mRevisionSuccess)
}

// HandleUserReport runs the end-to-end runner initiated by a user-report API
// request.
func (c *Controller) HandleUserReport() http.Handler {
	cfg := *c.config
	cfg.DoRevise = false
	cfg.DoUserReport = true
	return c.handleEndToEnd(&cfg, mUserReportSuccess)
}

// handleEndToEnd handles the common end-to-end scenario. m is incremented iff
// the run succeeds.
func (c *Controller) handleEndToEnd(cfg *config.E2ERunnerConfig, m *stats.Int64Measure) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		if cfg.DoUserReport {
			// no audit record for e2e test cleaning up after itself.
			if err := c.db.DeleteUserReport(project.TestPhoneNumber, database.NullActor); err != nil {
				logger.Errorw("error deleting previous user report for test phone number", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, err)
				return
			}
		}

		if err := clients.RunEndToEnd(ctx, cfg); err != nil {
			logger.Errorw("failure", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, m.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}

// HandleENXRedirect handles tests for the redirector service.
func (c *Controller) HandleENXRedirect() http.Handler {
	// If the client doesn't exist, it means the host was not provided.
	if c.client == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.h.RenderJSON(w, http.StatusOK, nil)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		if err := c.client.RunE2E(ctx); err != nil {
			logger.Errorw("failure", "error", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		stats.Record(ctx, mRedirectSuccess.M(1))
		c.h.RenderJSON(w, http.StatusOK, nil)
	})
}
