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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

const maxBatchSize = 10

// HandleBatchIssueAPI responds to the /batch-issue API for issuing verification codes
func (c *Controller) HandleBatchIssueAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		startTime := time.Now()
		if result := c.BatchIssueWithAPIAuth(w, r); result != nil {
			recordObservability(r.Context(), startTime, result)
		}
	})
}

// HandleBatchIssueUI responds to the /batch-issue API for issuing verification codes
func (c *Controller) HandleBatchIssueUI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		startTime := time.Now()
		if result := c.BatchIssueWithUIAuth(w, r); result != nil {
			recordObservability(r.Context(), startTime, result)
		}
	})
}

func (c *Controller) BatchIssueWithAPIAuth(w http.ResponseWriter, r *http.Request) *IssueResult {
	ctx := r.Context()
	result := &IssueResult{
		HTTPCode:  http.StatusOK,
		obsResult: enobs.ResultOK,
	}

	if authorizedApp := controller.AuthorizedAppFromContext(ctx); authorizedApp == nil {
		result.obsResult = enobs.ResultError("MISSING_AUTHORIZED_APP")
		controller.MissingAuthorizedApp(w, r, c.h)
		return result
	}

	c.decodeAndBulkIssue(ctx, w, r, result)
	return result
}

func (c *Controller) BatchIssueWithUIAuth(w http.ResponseWriter, r *http.Request) *IssueResult {
	ctx := r.Context()
	result := &IssueResult{
		HTTPCode:  http.StatusOK,
		obsResult: enobs.ResultOK,
	}

	membership := controller.MembershipFromContext(ctx)
	if !membership.Can(rbac.CodeBulkIssue) {
		result.obsResult = enobs.ResultError("BULK_ISSUE_NOT_ALLOWED")
		controller.Unauthorized(w, r, c.h)
		return result
	}
	ctx = controller.WithRealm(ctx, membership.Realm)

	c.decodeAndBulkIssue(ctx, w, r, result)
	return result
}

func (c *Controller) decodeAndBulkIssue(ctx context.Context, w http.ResponseWriter, r *http.Request, result *IssueResult) {
	// Ensure bulk upload is enabled on this realm.
	if currentRealm := controller.RealmFromContext(ctx); currentRealm == nil || !currentRealm.AllowBulkUpload {
		result.obsResult = enobs.ResultError("BULK_ISSUE_NOT_ENABLED")
		c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("bulk issuing is not enabled on this realm"))
		return
	}

	var request api.BatchIssueCodeRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		result.obsResult = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
		c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
		return
	}

	l := len(request.Codes)
	if l > maxBatchSize {
		result.obsResult = enobs.ResultError("BATCH_SIZE_LIMIT_EXCEEDED")
		c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("batch size limit [%d] exceeded", maxBatchSize))
		return
	}

	results := c.IssueMany(ctx, request.Codes)

	HTTPCode := http.StatusOK
	batchResp := &api.BatchIssueCodeResponse{}
	batchResp.Codes = make([]*api.IssueCodeResponse, len(results))
	errCount := 0

	for i, result := range results {
		singleResponse := result.IssueCodeResponse()
		batchResp.Codes[i] = singleResponse
		singleResponse.Padding = []byte{} // prevent inner padding of each response
		if singleResponse.Error == "" {
			continue
		}

		// If any issuance fails, the returned code is the code of the first failure
		// and continue processing all codes.
		errCount++
		if HTTPCode == http.StatusOK {
			HTTPCode = result.HTTPCode
			batchResp.ErrorCode = singleResponse.ErrorCode
		}
	}

	if errCount > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Failed to issue %d codes.", errCount))
		if succeeded := len(request.Codes) - errCount; succeeded > 0 {
			sb.WriteString(fmt.Sprintf(" Issued %d codes successfully.", succeeded))
		}
		sb.WriteString("See each error status in the codes array.")
		batchResp.Error = sb.String()
	}

	c.h.RenderJSON(w, HTTPCode, batchResp)
	return
}
