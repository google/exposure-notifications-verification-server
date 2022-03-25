// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"
)

// HandleIssueAPI responds to the /issue API for issuing verification codes
func (c *Controller) HandleIssueAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		realm := controller.RealmFromContext(ctx)

		if c.config.IsMaintenanceMode() || realm.MaintenanceMode {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		startTime := time.Now()
		if result := c.IssueWithAPIAuth(w, r); result != nil {
			recordObservability(r.Context(), startTime, result)
		}
	})
}

// HandleIssueUI responds to the /issue API for issuing verification codes
func (c *Controller) HandleIssueUI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		realm := controller.RealmFromContext(ctx)

		if c.config.IsMaintenanceMode() || realm.MaintenanceMode {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		startTime := time.Now()
		if result := c.IssueWithUIAuth(w, r); result != nil {
			recordObservability(r.Context(), startTime, result)
		}
	})
}

func (c *Controller) IssueWithAPIAuth(w http.ResponseWriter, r *http.Request) *IssueResult {
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

	c.decodeAndIssue(ctx, w, r, result)
	return result
}

func (c *Controller) IssueWithUIAuth(w http.ResponseWriter, r *http.Request) *IssueResult {
	ctx := r.Context()
	result := &IssueResult{
		HTTPCode:  http.StatusOK,
		obsResult: enobs.ResultOK,
	}

	membership := controller.MembershipFromContext(ctx)
	if !membership.Can(rbac.CodeIssue) {
		result.obsResult = enobs.ResultError("ISSUE_NOT_ALLOWED")
		controller.Unauthorized(w, r, c.h)
		return result
	}
	ctx = controller.WithRealm(ctx, membership.Realm)

	c.decodeAndIssue(ctx, w, r, result)
	return result
}

func (c *Controller) decodeAndIssue(ctx context.Context, w http.ResponseWriter, r *http.Request, result *IssueResult) {
	var request api.IssueCodeRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		result.obsResult = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
		c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
		return
	}

	realm := controller.RealmFromContext(ctx)
	if realm.MaintenanceMode {
		c.h.RenderJSON(w, http.StatusTooManyRequests,
			api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
		return
	}

	// If the report type is user-report AND the SMS template is not being set, change it to the user report template.
	if request.TestType == api.TestTypeUserReport && request.SMSTemplateLabel == "" {
		request.SMSTemplateLabel = database.UserReportTemplateLabel
	}

	internalRequest := &IssueRequestInternal{
		IssueRequest: &request,
	}

	res := c.IssueOne(ctx, internalRequest)
	result.HTTPCode = res.HTTPCode

	switch res.HTTPCode {
	case http.StatusInternalServerError:
		controller.InternalError(w, r, c.h, errors.New(res.ErrorReturn.Error))
		return
	case http.StatusOK:
		c.h.RenderJSON(w, http.StatusOK, res.IssueCodeResponse())
		return
	case http.StatusConflict:
		// This only occurs on "user-report" types where the phone number collides with
		// an already known one. In this case we just return "success" and don't
		// actually do anything. This matches the other user-report code issue paths.
		c.h.RenderJSON(w, http.StatusOK, &api.UserReportResponse{
			ExpiresAt:          res.IssueCodeResponse().ExpiresAt,
			ExpiresAtTimestamp: res.IssueCodeResponse().ExpiresAtTimestamp,
		})
		return
	default:
		c.h.RenderJSON(w, res.HTTPCode, res.ErrorReturn)
		return
	}
}
