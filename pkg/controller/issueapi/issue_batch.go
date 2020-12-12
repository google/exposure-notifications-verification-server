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
	"sync"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/hashicorp/go-multierror"
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
		result := &issueResult{
			httpCode:  http.StatusOK,
			obsBlame:  observability.BlameNone,
			obsResult: observability.ResultOK(),
		}
		defer recordObservability(ctx, result)

		var request api.BatchIssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		_, _, err := c.getAuthorizationFromContext(r)
		if err != nil {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("MISSING_AUTHORIZED_APP")
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		if realm := controller.RealmFromContext(ctx); !realm.AllowBulkUpload {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("BULK_ISSUE_NOT_ENABLED")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("bulk issuing is not enabled on this realm"))
			return
		}

		l := len(request.Codes)
		if l > maxBatchSize {
			result.obsBlame = observability.BlameClient
			result.obsResult = observability.ResultError("BATCH_SIZE_LIMIT_EXCEEDED")
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("batch size limit [%d] exceeded", maxBatchSize))
			return
		}

		resp.Codes = make([]*api.IssueCodeResponse, l)
		errorCh := make(chan *StatusAndError, len(request.Codes))
		var wg sync.WaitGroup
		for i, singleIssue := range request.Codes {
			wg.Add(1)
			go func(i int, singleIssue *api.IssueCodeRequest) {
				defer wg.Done()

				result, resp.Codes[i] = c.issue(ctx, singleIssue)
				if result.errorReturn != nil {
					// continue processing if when a single code issuance fails.
					// if any issuance fails, the returned code is the code of the first failure.
					logger.Warnw("single code issuance failed: %v", result.errorReturn)

					errorCh <- &StatusAndError{
						HTTPStatus: result.httpCode,
						Error:      result.errorReturn.Error,
					}
					if resp.Codes[i] == nil {
						resp.Codes[i] = &api.IssueCodeResponse{}
					}
					resp.Codes[i].ErrorCode = result.errorReturn.ErrorCode
					resp.Codes[i].Error = result.errorReturn.Error
				}
			}(i, singleIssue)
		}

		// wait for the work group to finish
		wg.Wait()

		httpCode := http.StatusOK
		var merr *multierror.Error
		// replay error channel
		for {
			select {
			case report := <-errorCh:
				if httpCode != http.StatusOK {
					httpCode = report.HTTPStatus
				}
				merr = multierror.Append(merr, errors.New(result.errorReturn.Error))
			default:
				break
			}
		}

		if err := merr.ErrorOrNil(); err != nil {
			resp.Error = err.Error()
		}

		c.h.RenderJSON(w, httpCode, resp)
		return
	})
}

type StatusAndError struct {
	HTTPStatus int
	Error      string
}
