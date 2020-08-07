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

package realmadmin

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Name string `form:"name,required"`

		TwilioAccountSid string `form:"twilio_account_sid"`
		TwilioAuthToken  string `form:"twilio_auth_token"`
		TwilioFromNumber string `form:"twilio_from_number"`
	}

	logger := c.logger.Named("HandleSave")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
			return
		}

		// SMS config is all or nothing
		if (form.TwilioAccountSid != "" || form.TwilioAuthToken != "" || form.TwilioFromNumber != "") &&
			(form.TwilioAccountSid == "" || form.TwilioAuthToken == "" || form.TwilioFromNumber == "") {
			flash.Error("Error updating realm: either all SMS fields must be specified or no SMS fields must be specified")
			http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
			return
		}

		// Process general settings
		realm.Name = form.Name
		if err := c.db.SaveRealm(realm); err != nil {
			logger.Errorw("failed to save realm", "error", err)
			flash.Error("Error updating realm: %v", err)
			http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
			return
		}

		// Process SMS settings
		smsConfig, err := realm.SMSConfig()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if smsConfig != nil {
			// We have an existing record
			if form.TwilioAccountSid == "" && form.TwilioAuthToken == "" && form.TwilioFromNumber == "" {
				// All fields are empty, delete the record
				if err := c.db.DeleteSMSConfig(smsConfig); err != nil {
					logger.Errorw("failed to delete smsConfig", "error", err)
					flash.Error("Error updating realm: %v", err)
					http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
					return
				}
			} else {
				// Potential updates
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				smsConfig.TwilioAuthToken = form.TwilioAuthToken
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					logger.Errorw("failed to update smsConfig", "error", err)
					flash.Error("Error updating realm: %v", err)
					http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
					return
				}
			}
		} else {
			// No SMS config exists
			if form.TwilioAccountSid != "" || form.TwilioAuthToken != "" || form.TwilioFromNumber != "" {
				// Values were provided
				smsConfig = &database.SMSConfig{
					RealmID:          realm.ID,
					ProviderType:     sms.ProviderTypeTwilio,
					TwilioAccountSid: form.TwilioAccountSid,
					TwilioAuthToken:  form.TwilioAuthToken,
					TwilioFromNumber: form.TwilioFromNumber,
				}

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					logger.Errorw("failed to create smsConfig", "error", err)
					flash.Error("Error updating realm: %v", err)
					http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
					return
				}
			}
		}

		flash.Alert("Updated realm settings!")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}
