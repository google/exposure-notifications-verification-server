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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// HandleSMSUpdate creates or updates the SMS config.
func (c *Controller) HandleSMSUpdate() http.Handler {
	type FormData struct {
		TwilioAccountSid string `form:"twilio_account_sid"`
		TwilioAuthToken  string `form:"twilio_auth_token"`
		TwilioFromNumber string `form:"twilio_from_number"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		smsConfig, err := c.db.SystemSMSConfig()
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			smsConfig = new(database.SMSConfig)
			smsConfig.IsSystem = true
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderShowSMS(ctx, w, smsConfig)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShowSMS(ctx, w, smsConfig)
			return
		}

		// Update
		smsConfig.TwilioAccountSid = form.TwilioAccountSid
		smsConfig.TwilioAuthToken = form.TwilioAuthToken
		smsConfig.TwilioFromNumber = form.TwilioFromNumber
		if err := c.db.SaveSMSConfig(smsConfig); err != nil {
			flash.Error("Failed to save system SMS config: %v", err)
			c.renderShowSMS(ctx, w, smsConfig)
			return
		}

		flash.Alert("Successfully updated system SMS config")
		http.Redirect(w, r, "/admin/sms", http.StatusSeeOther)
	})
}

func (c *Controller) renderShowSMS(ctx context.Context, w http.ResponseWriter, smsConfig *database.SMSConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m["smsConfig"] = smsConfig
	c.h.RenderHTML(w, "admin/sms/show", m)
}
