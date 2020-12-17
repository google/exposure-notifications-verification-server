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
	"fmt"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi/issuelogic"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

const maxBatchSize = 10

// HandleBatchIssue shows the page for batch-issuing codes.
func (c *Controller) HandleBatchIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("issueapi.HandleBatchIssue")

		resp := &api.BatchIssueCodeResponse{}
		result := &issuelogic.IssueResult{
			HttpCode:  http.StatusOK,
			ObsBlame:  observability.BlameNone,
			ObsResult: observability.ResultOK(),
		}
		defer recordObservability(ctx, result)

		var request api.BatchIssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		authApp, membership, realm, err := c.getAuthorizationFromContext(ctx)
		if err != nil {
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("MISSING_AUTHORIZED_APP")
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		// Ensure bulk upload is enabled on this realm.
		if !realm.AllowBulkUpload {
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("BULK_ISSUE_NOT_ENABLED")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("bulk issuing is not enabled on this realm"))
			return
		}

		if membership != nil && !membership.Can(rbac.CodeBulkIssue) {
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("BULK_ISSUE_NOT_ENABLED")
			controller.Unauthorized(w, r, c.h)
			return
		}

		l := len(request.Codes)
		if l > maxBatchSize {
			result.ObsBlame = observability.BlameClient
			result.ObsResult = observability.ResultError("BATCH_SIZE_LIMIT_EXCEEDED")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("batch size limit [%d] exceeded", maxBatchSize))
			return
		}

		httpCode := http.StatusOK
		errCount := 0
		var results []*issuelogic.IssueResult

		logic := issuelogic.New(c.config, c.db, c.limiter, authApp, membership, realm)
		results, resp.Codes = logic.IssueMany(ctx, request.Codes)
		for _, result := range results {
			if result.ErrorReturn == nil {
				continue
			}

			// if any issuance fails, the returned code is the code of the first failure
			// and  continue processing all codes.
			logger.Warnw("single code issuance failed", "error", result.ErrorReturn)
			errCount++
			if httpCode == http.StatusOK {
				httpCode = result.HttpCode
				resp.ErrorCode = result.ErrorReturn.ErrorCode
			}
		}

		if errCount > 0 {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Failed to issue %d codes.", errCount))
			if succeeded := len(request.Codes) - errCount; succeeded > 0 {
				sb.WriteString(fmt.Sprintf(" Issued %d codes successfully.", succeeded))
			}
			sb.WriteString("See each error status in the codes array.")
			resp.Error = sb.String()
		}

		c.h.RenderJSON(w, httpCode, resp)
		return
	})
}
