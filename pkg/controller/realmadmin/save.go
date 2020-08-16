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
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Name             string            `form:"name"`
		AllowedTestTypes database.TestType `form:"allowedTestTypes"`

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

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderShow(ctx, w, realm, nil)
			return
		}

		// SMS config is all or nothing
		if (form.TwilioAccountSid != "" || form.TwilioAuthToken != "" || form.TwilioFromNumber != "") &&
			(form.TwilioAccountSid == "" || form.TwilioAuthToken == "" || form.TwilioFromNumber == "") {
			flash.Error("Error updating realm: either all SMS fields must be specified or no SMS fields must be specified")
			c.renderShow(ctx, w, realm, nil)
			return
		}

		// Process general settings
		realm.Name = form.Name
		realm.AllowedTestTypes = form.AllowedTestTypes
		if err := c.db.SaveRealm(realm); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderShow(ctx, w, realm, nil)
			return
		}

		// Process SMS settings
		smsConfig, err := realm.SMSConfig(c.db)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}
		if smsConfig != nil {
			// We have an existing record
			if form.TwilioAccountSid == "" && form.TwilioAuthToken == "" && form.TwilioFromNumber == "" {
				// All fields are empty, delete the record
				if err := c.db.DeleteSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderShow(ctx, w, realm, smsConfig)
					return
				}
			} else {
				// Potential updates
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				smsConfig.TwilioAuthToken = form.TwilioAuthToken
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderShow(ctx, w, realm, smsConfig)
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
					flash.Error("Failed to update realm: %v", err)
					c.renderShow(ctx, w, realm, smsConfig)
					return
				}
			}
		}

		flash.Alert("Successfully updated realm settings!")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, realm *database.Realm, smsConfig *database.SMSConfig) {
	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm
	m["smsConfig"] = smsConfig
	m["testTypes"] = map[string]database.TestType{
		"confirmed": database.TestTypeConfirmed,
		"likely":    database.TestTypeConfirmed | database.TestTypeLikely,
		"negative":  database.TestTypeConfirmed | database.TestTypeLikely | database.TestTypeNegative,
	}
	c.h.RenderHTML(w, "realm", m)
}
