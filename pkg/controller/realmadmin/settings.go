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
	shortCodeLengths            = []int{6, 7, 8}
	shortCodeMinutes            = []int{}
	longCodeLengths             = []int{12, 13, 14, 15, 16}
	longCodeHours               = []int{}
	passwordRotationPeriodDays  = []int{0, 30, 60, 90, 365}
	passwordRotationWarningDays = []int{0, 1, 3, 5, 7, 30}
)

func init() {
	for i := 5; i <= 60; i++ {
		shortCodeMinutes = append(shortCodeMinutes, i)
	}
	for i := 1; i <= 24; i++ {
		longCodeHours = append(longCodeHours, i)
	}
}

func (c *Controller) HandleSettings() http.Handler {
	type FormData struct {
		Name           string `form:"name"`
		RegionCode     string `form:"region_code"`
		WelcomeMessage string `form:"welcome_message"`

		AllowedTestTypes      database.TestType `form:"allowed_test_types"`
		RequireDate           *bool             `form:"require_date"`
		CodeLength            uint              `form:"code_length"`
		CodeDurationMinutes   int64             `form:"code_duration"`
		LongCodeLength        uint              `form:"long_code_length"`
		LongCodeDurationHours int64             `form:"long_code_duration"`
		SMSTextTemplate       string            `form:"sms_text_template"`

		TwilioAccountSid string `form:"twilio_account_sid"`
		TwilioAuthToken  string `form:"twilio_auth_token"`
		TwilioFromNumber string `form:"twilio_from_number"`

		MFAMode                     *int16 `form:"mfa_mode"`
		EmailVerifiedMode           *int16 `form:"email_verified_mode"`
		PasswordRotationPeriodDays  uint   `form:"password_rotation_period_days"`
		PasswordRotationWarningDays uint   `form:"password_rotation_warning_days"`

		AbusePreventionEnabled     *bool   `form:"abuse_prevention_enabled"`
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

		if r.Method == http.MethodGet {
			c.renderSettings(ctx, w, r, realm, nil)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderSettings(ctx, w, r, realm, nil)
			return
		}

		// SMS config is all or nothing
		if (form.TwilioAccountSid != "" || form.TwilioAuthToken != "" || form.TwilioFromNumber != "") &&
			(form.TwilioAccountSid == "" || form.TwilioAuthToken == "" || form.TwilioFromNumber == "") {
			flash.Error("Error updating realm: either all SMS fields must be specified or no SMS fields must be specified")
			c.renderSettings(ctx, w, r, realm, nil)
			return
		}

		// General
		if v := form.Name; v != "" {
			realm.Name = v
		}
		if v := form.RegionCode; v != "" {
			realm.RegionCode = v
		}
		if v := form.WelcomeMessage; v != "" {
			realm.WelcomeMessage = v
		}

		// Codes
		if v := form.AllowedTestTypes; v > 0 {
			realm.AllowedTestTypes = v
		}
		if v := form.RequireDate; v != nil {
			realm.RequireDate = *v
		}
		if v := form.CodeLength; v > 0 {
			realm.CodeLength = v
		}
		if v := form.CodeDurationMinutes; v > 0 {
			realm.CodeDuration.Duration = time.Duration(v) * time.Minute
		}
		if v := form.LongCodeLength; v > 0 {
			realm.LongCodeLength = v
		}
		if v := form.LongCodeDurationHours; v > 0 {
			realm.LongCodeDuration.Duration = time.Duration(v) * time.Minute
		}
		if v := form.SMSTextTemplate; v != "" {
			realm.SMSTextTemplate = v
		}

		// Security
		if v := form.EmailVerifiedMode; v != nil {
			realm.EmailVerifiedMode = database.AuthRequirement(*v)
		}
		if v := form.MFAMode; v != nil {
			realm.MFAMode = database.AuthRequirement(*v)
		}
		if v := form.PasswordRotationPeriodDays; v > 0 {
			realm.PasswordRotationPeriodDays = v
		}
		if v := form.PasswordRotationWarningDays; v > 0 {
			realm.PasswordRotationWarningDays = v
		}

		// Abuse prevention
		if v := form.AbusePreventionEnabled; v != nil {
			realm.AbusePreventionEnabled = *v
		}
		if v := form.AbusePreventionLimitFactor; v > 0 {
			realm.AbusePreventionLimitFactor = v
		}

		// Save
		if err := c.db.SaveRealm(realm); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderSettings(ctx, w, r, realm, nil)
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
					c.renderSettings(ctx, w, r, realm, smsConfig)
					return
				}
			} else {
				// Potential updates
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				smsConfig.TwilioAuthToken = form.TwilioAuthToken
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, realm, smsConfig)
					return
				}
			}
		} else {
			// No SMS config exists
			if form.TwilioAccountSid != "" && form.TwilioAuthToken != "" && form.TwilioFromNumber != "" {
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
					c.renderSettings(ctx, w, r, realm, smsConfig)
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

func (c *Controller) renderSettings(ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm, smsConfig *database.SMSConfig) {
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
	// Valid settings for pwd rotation.
	m["passwordRotateDays"] = passwordRotationPeriodDays
	m["passwordWarnDays"] = passwordRotationWarningDays

	// Valid settings for code parameters.
	m["shortCodeLengths"] = shortCodeLengths
	m["shortCodeMinutes"] = shortCodeMinutes
	m["longCodeLengths"] = longCodeLengths
	m["longCodeHours"] = longCodeHours
	c.h.RenderHTML(w, "realmadmin/edit", m)
}
