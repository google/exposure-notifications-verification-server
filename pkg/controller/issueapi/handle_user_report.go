// Copyright 2021 Google LLC
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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
)

func (c *Controller) HandleUserReport() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.IsMaintenanceMode() {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("userreportapi.HandleUserReport")

		blame := enobs.BlameNone
		result := enobs.ResultOK
		defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &blame)

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			blame = enobs.BlameClient
			result = enobs.ResultError("MISSING_AUTHORIZED_APP")
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		// Parse the UserReportRequest struct.
		var request api.UserReportRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("bad request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}

		// Ensure realm allows user report.
		realm := controller.RealmFromContext(ctx)
		if !realm.AllowsUserReport() {
			logger.Warnw("realm is requesting user report, but disabled", "realmID", realm.ID)
			blame = enobs.BlameClient
			result = enobs.ResultError("USER_REPORT_NOT_ENABLED")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("user initiated report is not enabled").WithCode(api.ErrUnsupportedTestType))
			return
		}

		nonce, err := base64util.DecodeString(request.Nonce)
		if err != nil {
			logger.Errorw("bad request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}
		if len(nonce) == 0 {
			logger.Errorw("bad request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("USER_REQUEST_MISSING_NONCE")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("nonce cannot be empty").WithCode(api.ErrMissingNonce))
			return
		}

		if len(request.Phone) == 0 {
			logger.Errorw("bad request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("USER_REQUEST_MISSING_PHONE")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("phone cannot be empty").WithCode(api.ErrMissingPhone))
			return
		}

		// Issue code and send text.
		issueRequest := &IssueRequestInternal{
			IssueRequest: &api.IssueCodeRequest{
				SymptomDate:      request.SymptomDate,
				TestDate:         request.TestDate,
				TestType:         api.TestTypeUserReport, // Always test type of user report.
				Phone:            request.Phone,
				SMSTemplateLabel: database.UserReportTemplateLabel,
			},
			UserRequested: true,
			Nonce:         nonce,
		}

		res := c.IssueOne(ctx, issueRequest)

		switch res.HTTPCode {
		case http.StatusInternalServerError:
			controller.InternalError(w, r, c.h, errors.New(res.ErrorReturn.Error))
			return
		case http.StatusConflict:
			c.h.RenderJSON(w, http.StatusOK, &api.UserReportResponse{
				ExpiresAt:          res.IssueCodeResponse().ExpiresAt,
				ExpiresAtTimestamp: res.IssueCodeResponse().ExpiresAtTimestamp,
			})
			return
		case http.StatusOK:
			c.h.RenderJSON(w, http.StatusOK, &api.UserReportResponse{
				ExpiresAt:          res.IssueCodeResponse().ExpiresAt,
				ExpiresAtTimestamp: res.IssueCodeResponse().ExpiresAtTimestamp,
			})
			return
		default:
			c.h.RenderJSON(w, res.HTTPCode, res.ErrorReturn)
			return
		}
	})
}
