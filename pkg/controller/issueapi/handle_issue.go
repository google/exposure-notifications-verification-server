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
	"errors"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
)

func (c *Controller) HandleIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		ctx := r.Context()
		result := &issueResult{
			httpCode:  http.StatusOK,
			obsResult: observability.ResultOK(),
		}
		defer recordObservability(ctx, result)

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			result.obsResult = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}

		authApp, membership, realm, err := c.getAuthorizationFromContext(ctx)
		if err != nil {
			result.obsResult = observability.ResultError("MISSING_AUTHORIZED_APP")
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		result = c.issueOne(ctx, &request, authApp, membership, realm)
		resp := result.issueCodeResponse()
		if resp.Error != "" {
			if result.httpCode == http.StatusInternalServerError {
				controller.InternalError(w, r, c.h, errors.New(resp.Error))
				return
			}
			c.h.RenderJSON(w, result.httpCode, resp)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, resp)
	})
}