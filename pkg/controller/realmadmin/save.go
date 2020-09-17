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
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

var (
	shortCodeLengths = []int{6, 7, 8}
	shortCodeMinutes = []int{}
	longCodeLengths  = []int{12, 13, 14, 15, 16}
	longCodeHours    = []int{}
)

func init() {
	for i := 5; i <= 60; i++ {
		shortCodeMinutes = append(shortCodeMinutes, i)
	}
	for i := 1; i <= 24; i++ {
		longCodeHours = append(longCodeHours, i)
	}
}

func (c *Controller) HandleSave() http.Handler {
	type FormData struct {
		Name             string            `form:"name"`
		RegionCode       string            `form:"regionCode"`
		AllowedTestTypes database.TestType `form:"allowedTestTypes"`
		MFAMode          int16             `form:"MFAMode"`
		RequireDate      bool              `form:"requireDate"`

		CodeLength          uint   `form:"codeLength"`
		CodeDurationMinutes int64  `form:"codeDuration"`
		LongCodeLength      uint   `form:"longCodeLength"`
		LongCodeHours       int64  `form:"longCodeDuration"`
		SMSTextTemplate     string `form:"SMSTextTemplate"`

		TwilioAccountSid string `form:"twilio_account_sid"`
		TwilioAuthToken  string `form:"twilio_auth_token"`
		TwilioFromNumber string `form:"twilio_from_number"`

		AbusePreventionEnabled     bool    `form:"abuse_prevention_enabled"`
		AbusePreventionLimitFactor float32 `form:"abuse_prevention_limit_factor"`
		AbusePreventionBurst       uint64  `form:"abuse_prevention_burst"`
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
			c.renderShow(ctx, w, r, realm, nil)
			return
		}

		// SMS config is all or nothing
		if (form.TwilioAccountSid != "" || form.TwilioAuthToken != "" || form.TwilioFromNumber != "") &&
			(form.TwilioAccountSid == "" || form.TwilioAuthToken == "" || form.TwilioFromNumber == "") {
			flash.Error("Error updating realm: either all SMS fields must be specified or no SMS fields must be specified")
			c.renderShow(ctx, w, r, realm, nil)
			return
		}

		// Process general settings
		realm.Name = form.Name
		realm.RegionCode = form.RegionCode
		realm.AllowedTestTypes = form.AllowedTestTypes
		realm.RequireDate = form.RequireDate
		realm.CodeLength = form.CodeLength
		realm.CodeDuration.Duration = time.Duration(form.CodeDurationMinutes) * time.Minute
		realm.LongCodeLength = form.LongCodeLength
		realm.LongCodeDuration.Duration = time.Hour * time.Duration(form.LongCodeHours)
		realm.SMSTextTemplate = form.SMSTextTemplate
		realm.MFAMode = database.MFAMode(form.MFAMode)
		realm.AbusePreventionEnabled = form.AbusePreventionEnabled
		realm.AbusePreventionLimitFactor = form.AbusePreventionLimitFactor
		if err := c.db.SaveRealm(realm); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderShow(ctx, w, r, realm, nil)
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
					c.renderShow(ctx, w, r, realm, smsConfig)
					return
				}
			} else {
				// Potential updates
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				smsConfig.TwilioAuthToken = form.TwilioAuthToken
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderShow(ctx, w, r, realm, smsConfig)
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
					c.renderShow(ctx, w, r, realm, smsConfig)
					return
				}
			}
		}

		// Process temporary abuse prevention bursts
		if burst := form.AbusePreventionBurst; burst > 0 {
			dig, err := digest.HMACUint(realm.ID, c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			key := fmt.Sprintf("realm:quota:%s", dig)
			if err := c.limiter.Burst(ctx, key, burst); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			flash.Alert("Successfully added %d to realm quota!", burst)
		}

		flash.Alert("Successfully updated realm settings!")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}

func (c *Controller) renderShow(ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm, smsConfig *database.SMSConfig) {
	if smsConfig == nil {
		var err error
		smsConfig, err = realm.SMSConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			smsConfig = new(database.SMSConfig)
		}
	}

	m := controller.TemplateMapFromContext(ctx)
	m["realm"] = realm
	m["smsConfig"] = smsConfig
	m["testTypes"] = map[string]database.TestType{
		"confirmed": database.TestTypeConfirmed,
		"likely":    database.TestTypeConfirmed | database.TestTypeLikely,
		"negative":  database.TestTypeConfirmed | database.TestTypeLikely | database.TestTypeNegative,
	}
	// Valid settings for code parameters.
	m["shortCodeLengths"] = shortCodeLengths
	m["shortCodeMinutes"] = shortCodeMinutes
	m["longCodeLengths"] = longCodeLengths
	m["longCodeHours"] = longCodeHours
	c.h.RenderHTML(w, "realm", m)
}
