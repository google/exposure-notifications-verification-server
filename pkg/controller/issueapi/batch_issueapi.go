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

package issueapi

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
)

// HandleBatchIssue shows the page for batch-issuing codes.
func (c *Controller) HandleBatchIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		ctx := r.Context()
		realm := controller.RealmFromContext(ctx)
		if !realm.AllowBulkUpload {
			controller.Unauthorized(w, r, c.h)
			return
		}

		hasSMSConfig, err := realm.HasSMSConfig(c.db)
		if err != nil || !hasSMSConfig {
			controller.InternalError(w, r, c.h, err)
			return
		}

		result := &issueResult{
			httpCode:  http.StatusOK,
			obsBlame:  observability.BlameNone,
			obsResult: observability.ResultOK(),
		}
		defer observability.RecordLatency(&ctx, time.Now(), mLatencyMs, &result.obsBlame, &result.obsResult)

		var request api.BatchIssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			result.obsBlame, result.obsResult = observability.BlameClient, observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			return
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			result.obsBlame, result.obsResult = observability.BlameClient, observability.ResultError("MISSING_AUTHORIZED_APP")
			return
		}

		if authApp != nil {
			realm, err = authApp.Realm(c.db)
			if err != nil {
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				result.obsBlame, result.obsResult = observability.BlameClient, observability.ResultError("UNAUTHORIZED")
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
		}
		if realm == nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
			result.obsBlame, result.obsResult = observability.BlameServer, observability.ResultError("MISSING_REALM")
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		ctx = observability.WithRealmID(ctx, realm.ID)

		for _, singleIssue := range request.Codes {
			resp := c.issue(ctx, singleIssue, realm, user, authApp, result)
		}

		c.h.RenderJSON(w, http.StatusOK, resp)
		return
	})
}
