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

// HandleIssue responds to the /issue API for issuing verification codes
func (c *Controller) HandleIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if result := c.HandleIssueFn(w, r); result != nil {
			recordObservability(r.Context(), result)
		}
	})
}

func (c *Controller) HandleIssueFn(w http.ResponseWriter, r *http.Request) *IssueResult {
	if c.config.IsMaintenanceMode() {
		c.h.RenderJSON(w, http.StatusTooManyRequests,
			api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
		return nil
	}

	ctx := r.Context()
	result := &IssueResult{
		HTTPCode:  http.StatusOK,
		obsResult: observability.ResultOK(),
	}

	var request api.IssueCodeRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		result.obsResult = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
		c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
		return result
	}

	authApp, membership, realm, err := c.getAuthorizationFromContext(ctx)
	if err != nil {
		result.obsResult = observability.ResultError("MISSING_AUTHORIZED_APP")
		c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
		return result
	}

	res := c.IssueOne(ctx, &request, authApp, membership, realm)
	result.HTTPCode = res.HTTPCode
	resp := res.IssueCodeResponse()
	if resp.Error != "" {
		if result.HTTPCode == http.StatusInternalServerError {
			controller.InternalError(w, r, c.h, errors.New(resp.Error))
			return result
		}
		c.h.RenderJSON(w, result.HTTPCode, resp)
		return result
	}

	c.h.RenderJSON(w, http.StatusOK, resp)
	return result
}
