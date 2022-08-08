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

package admin

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

// HandleSMSUpdate creates or updates the SMS config.
func (c *Controller) HandleSMSUpdate() http.Handler {
	type FormDataFromNumber struct {
		ID    uint   `form:"id"`
		Label string `form:"label,required"`
		Value string `form:"value,required"`
	}

	type FormData struct {
		TwilioAccountSid string `form:"twilio_account_sid"`
		TwilioAuthToken  string `form:"twilio_auth_token"`

		TwilioFromNumbers []*FormDataFromNumber `form:"twilio_from_numbers"`
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

		smsFromNumbers, err := c.db.SMSFromNumbers()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Requested form, stop processing.
		if r.Method == http.MethodGet {
			c.renderShowSMS(ctx, w, smsConfig, smsFromNumbers)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			smsConfig.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShowSMS(ctx, w, smsConfig, smsFromNumbers)
			return
		}

		// Update Twilio config
		smsConfig.ProviderType = sms.ProviderTypeTwilio
		smsConfig.TwilioAccountSid = form.TwilioAccountSid
		if form.TwilioAuthToken != project.PasswordSentinel {
			smsConfig.TwilioAuthToken = form.TwilioAuthToken
		}
		if err := c.db.SaveSMSConfig(smsConfig); err != nil {
			flash.Error("Failed to save system SMS config: %v", err)
			c.renderShowSMS(ctx, w, smsConfig, smsFromNumbers)
			return
		}

		// Update from numbers
		updatedSMSFromNumbers := make([]*database.SMSFromNumber, 0, len(form.TwilioFromNumbers))
		for _, v := range form.TwilioFromNumbers {
			// People do weird things in multi-forms. Only accept the value if it's
			// completely intact.
			if v == nil || v.Label == "" || v.Value == "" {
				continue
			}

			var smsFromNumber database.SMSFromNumber
			smsFromNumber.ID = v.ID
			smsFromNumber.Label = v.Label
			smsFromNumber.Value = v.Value
			updatedSMSFromNumbers = append(updatedSMSFromNumbers, &smsFromNumber)
		}

		if err := c.db.CreateOrUpdateSMSFromNumbers(updatedSMSFromNumbers); err != nil {
			flash.Error("Failed to save system SMS from numbers: %s", err)
			c.renderShowSMS(ctx, w, smsConfig, smsFromNumbers)
			return
		}

		flash.Alert("Successfully updated system SMS config")
		http.Redirect(w, r, "/admin/sms", http.StatusSeeOther)
	})
}

func (c *Controller) renderShowSMS(ctx context.Context, w http.ResponseWriter,
	smsConfig *database.SMSConfig, smsFromNumbers []*database.SMSFromNumber,
) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("SMS - System Admin")
	m["smsConfig"] = smsConfig
	m["smsFromNumbers"] = smsFromNumbers
	c.h.RenderHTML(w, "admin/sms/show", m)
}
