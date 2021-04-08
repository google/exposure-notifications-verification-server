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

package userreport

import (
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/sessions"
)

func stringFromSession(session *sessions.Session, key string) (string, error) {
	v, ok := session.Values[key]
	if !ok {
		return "", fmt.Errorf("there is no %q in the session", key)
	}
	strVal, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("the session value for %q is not a string", key)
	}
	return strVal, nil
}

func (c *Controller) HandleSend() http.Handler {
	type FormData struct {
		TestDate    string `form:"testDate"`
		SymptomDate string `form:"symptomDate"`
		Phone       string `form:"phone"`
		Agreement   bool   `form:"agreement"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("userreport.HandleSend")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			logger.Debugw("missing session")
			controller.MissingSession(w, r, c.h)
			return
		}

		region, err := stringFromSession(session, regionKey)
		if err != nil {
			controller.NotFound(w, r, c.h)
			return
		}
		realm, err := c.db.FindRealmByRegion(region)
		if err != nil {
			if database.IsNotFound(err) {
				controller.NotFound(w, r, c.h)
				return
			}

			logger.Warnw("region doesn't exist", "region", region, "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}
		ctx = controller.WithRealm(ctx, realm)

		if !(realm.AllowsUserReport() && realm.AllowAdminUserReport) {
			controller.NotFound(w, r, c.h)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			// TODO(mikehelmick) : render form again and handle errors
			logger.Warn("error binding form", "error", err)
			controller.BadRequest(w, r, c.h)
			return
		}

		// Pull the nonce from the session.
		nonceStr, err := stringFromSession(session, nonceKey)
		if err != nil {
			controller.NotFound(w, r, c.h)
			return
		}
		nonce, err := base64util.DecodeString(nonceStr)
		if err != nil {
			logger.Warn("nonce cannot be decoded")
			controller.BadRequest(w, r, c.h)
			return
		}

		// Attempt to send the code.
		issueRequest := &issueapi.IssueRequestInternal{
			IssueRequest: &api.IssueCodeRequest{
				SymptomDate:      form.SymptomDate,
				TestDate:         form.TestDate,
				TestType:         api.TestTypeUserReport, // Always test type of user report.
				Phone:            form.Phone,
				SMSTemplateLabel: database.UserReportTemplateLabel,
			},
			UserRequested: true,
			Nonce:         nonce,
		}

		result := c.issueController.IssueOne(ctx, issueRequest)
		if result.HTTPCode != http.StatusOK {
			// TODO(mikehelmick) : render form again and handle errors if appropriate, displaying success may be appropriate.
			logger.Errorw("error issuing code", "result", result)
			controller.InternalError(w, r, c.h, fmt.Errorf("error issuing verification code: %v", result.ErrorReturn.Error))
		}

		m.Title("Request a verification code")
		m["realm"] = realm
		c.h.RenderHTML(w, "report/issue", m)
	})
}
