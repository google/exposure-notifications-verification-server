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
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleSend() http.Handler {
	type FormData struct {
		TestDate  string `form:"testDate"`
		Phone     string `form:"phone"`
		Agreement bool   `form:"agreement"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("userreport.HandleSend")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		region := controller.RegionFromSession(session)
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

		if !realm.AllowsUserReport() {
			controller.NotFound(w, r, c.h)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			logger.Warn("error binding form", "error", err)
			// TODO(mikehelmick): i18n
			m["error"] = []string{"Internal error, please try again"}
			c.renderIndex(w, realm, m)
			return
		}
		m["date"] = form.TestDate
		m["phoneNumber"] = form.Phone
		m["agreement"] = form.Agreement

		// Pull the nonce from the session.
		nonceStr := controller.NonceFromSession(session)
		if nonceStr == "" {
			controller.NotFound(w, r, c.h)
			return
		}
		nonce, err := base64util.DecodeString(nonceStr)
		if err != nil {
			logger.Warnw("nonce cannot be decoded", "error", err)
			// TODO(mikehelmick): i18n
			m["error"] = []string{"Internal error, please close your Exposure Notifications application and try again."}
			c.renderIndex(w, realm, m)
			return
		}

		// Check agreement.
		if !form.Agreement {
			// TODO(mikehelmick): i18n
			msg := "You must agree to the terms to request a verification code"
			m["error"] = []string{msg}
			m["termsError"] = msg
			c.renderIndex(w, realm, m)
			return
		}

		// Attempt to send the code.
		issueRequest := &issueapi.IssueRequestInternal{
			IssueRequest: &api.IssueCodeRequest{
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
			// Handle errors that the user can fix.
			// TODO(mikehelmick): i18n for messaging.
			if result.ErrorReturn.ErrorCode == api.ErrInvalidDate {
				m["error"] = []string{result.ErrorReturn.Error}
				m["dateError"] = result.ErrorReturn.Error
				c.renderIndex(w, realm, m)
				return
			}
			if result.ErrorReturn.ErrorCode == api.ErrMissingPhone {
				m["error"] = []string{result.ErrorReturn.Error}
				m["phoneError"] = result.ErrorReturn.Error
				c.renderIndex(w, realm, m)
				return
			}
			if result.ErrorReturn.ErrorCode == api.ErrQuotaExceeded {
				m["error"] = []string{result.ErrorReturn.Error}
				c.renderIndex(w, realm, m)
				return
			}

			// The error returned isn't something the user can easily fix, show internal error, but hide form.
			m["error"] = []string{"There was an internal error. A verification code cannot be requested at this time."}
			m["skipForm"] = true
			c.renderIndex(w, realm, m)
			return
		}

		controller.ClearNonceFromSession(session)

		m.Title("Request a verification code")
		m["realm"] = realm
		c.h.RenderHTML(w, "report/issue", m)
	})
}
