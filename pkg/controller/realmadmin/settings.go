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
	mfaGracePeriod              = []int64{0, 1, 7, 30}
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
		General        bool   `form:"general"`
		Name           string `form:"name"`
		RegionCode     string `form:"region_code"`
		WelcomeMessage string `form:"welcome_message"`

		Codes                 bool              `form:"codes"`
		AllowedTestTypes      database.TestType `form:"allowed_test_types"`
		RequireDate           bool              `form:"require_date"`
		CodeLength            uint              `form:"code_length"`
		CodeDurationMinutes   int64             `form:"code_duration"`
		LongCodeLength        uint              `form:"long_code_length"`
		LongCodeDurationHours int64             `form:"long_code_duration"`
		SMSTextTemplate       string            `form:"sms_text_template"`

		SMS                bool   `form:"sms"`
		UseSystemSMSConfig bool   `form:"use_system_sms_config"`
		SMSCountry         string `form:"sms_country"`
		TwilioAccountSid   string `form:"twilio_account_sid"`
		TwilioAuthToken    string `form:"twilio_auth_token"`
		TwilioFromNumber   string `form:"twilio_from_number"`

		Security                    bool   `form:"security"`
		MFAMode                     int16  `form:"mfa_mode"`
		MFARequiredGracePeriod      int64  `form:"mfa_grace_period"`
		EmailVerifiedMode           int16  `form:"email_verified_mode"`
		PasswordRotationPeriodDays  uint   `form:"password_rotation_period_days"`
		PasswordRotationWarningDays uint   `form:"password_rotation_warning_days"`
		AllowedCIDRsAdminAPI        string `form:"allowed_cidrs_adminapi"`
		AllowedCIDRsAPIServer       string `form:"allowed_cidrs_apiserver"`
		AllowedCIDRsServer          string `form:"allowed_cidrs_server"`

		AbusePrevention            bool    `form:"abuse_prevention"`
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

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		var quotaLimit, quotaRemaining uint64
		if realm.AbusePreventionEnabled {
			dig, err := digest.HMACUint(realm.ID, c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			key := fmt.Sprintf("realm:quota:%s", dig)

			quotaLimit, quotaRemaining, err = c.limiter.Get(ctx, key)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		if r.Method == http.MethodGet {
			c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
			return
		}

		// General
		if form.General {
			realm.Name = form.Name
			realm.RegionCode = form.RegionCode
			realm.WelcomeMessage = form.WelcomeMessage
		}

		// Codes
		if form.Codes {
			realm.AllowedTestTypes = form.AllowedTestTypes
			realm.RequireDate = form.RequireDate
			realm.SMSTextTemplate = form.SMSTextTemplate

			// These fields can only be set if ENX is disabled
			if !realm.EnableENExpress {
				realm.CodeLength = form.CodeLength
				realm.CodeDuration.Duration = time.Duration(form.CodeDurationMinutes) * time.Minute
				realm.LongCodeLength = form.LongCodeLength
				realm.LongCodeDuration.Duration = time.Duration(form.LongCodeDurationHours) * time.Hour
			}
		}

		// SMS
		if form.SMS {
			realm.UseSystemSMSConfig = form.UseSystemSMSConfig
			realm.SMSCountry = form.SMSCountry
		}

		// Security
		if form.Security {
			realm.EmailVerifiedMode = database.AuthRequirement(form.EmailVerifiedMode)
			realm.MFAMode = database.AuthRequirement(form.MFAMode)
			realm.MFARequiredGracePeriod = database.FromDuration(time.Duration(form.MFARequiredGracePeriod) * 24 * time.Hour)
			realm.PasswordRotationPeriodDays = form.PasswordRotationPeriodDays
			realm.PasswordRotationWarningDays = form.PasswordRotationWarningDays

			allowedCIDRsAdminADPI, err := database.ToCIDRList(form.AllowedCIDRsAdminAPI)
			if err != nil {
				realm.AddError("allowedCIDRsAdminAPI", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
				return
			}
			realm.AllowedCIDRsAdminAPI = allowedCIDRsAdminADPI

			allowedCIDRsAPIServer, err := database.ToCIDRList(form.AllowedCIDRsAPIServer)
			if err != nil {
				realm.AddError("allowedCIDRsAPIServer", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
				return
			}
			realm.AllowedCIDRsAPIServer = allowedCIDRsAPIServer

			allowedCIDRsServer, err := database.ToCIDRList(form.AllowedCIDRsServer)
			if err != nil {
				realm.AddError("allowedCIDRsServer", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
				return
			}
			realm.AllowedCIDRsServer = allowedCIDRsServer
		}

		// Abuse prevention
		var abusePreventionJustEnabled bool
		if form.AbusePrevention {
			abusePreventionJustEnabled = !realm.AbusePreventionEnabled && form.AbusePreventionEnabled

			realm.AbusePreventionEnabled = form.AbusePreventionEnabled
			realm.AbusePreventionLimitFactor = form.AbusePreventionLimitFactor
		}

		// If abuse prevention was just enabled, create the initial bucket so
		// enforcement works. We do this before actually saving the configuration on
		// the realm to avoid a race where someone is issuing a code where abuse
		// prevention has been enabled, but the quota has not been set. In that
		// case, the quota would be the "default" quota for the limiter, which is
		// not ideal or correct.
		//
		// Even if saving the realm fails, there's no harm in doing this early. It's
		// an idempotent operation that TTLs out after a week anyway.
		if abusePreventionJustEnabled {
			dig, err := digest.HMACUint(realm.ID, c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			key := fmt.Sprintf("realm:quota:%s", dig)
			limit := uint64(realm.AbusePreventionEffectiveLimit())
			ttl := 7 * 24 * time.Hour
			if err := c.limiter.Set(ctx, key, limit, ttl); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// Save realm
		if err := c.db.SaveRealm(realm, currentUser); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderSettings(ctx, w, r, realm, nil, quotaLimit, quotaRemaining)
			return
		}

		// SMS
		if form.SMS && !form.UseSystemSMSConfig {
			// Fetch the existing SMS config record, if one exists
			smsConfig, err := realm.SMSConfig(c.db)
			if err != nil && !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			if smsConfig != nil && !smsConfig.IsSystem {
				// We have an existing record and the existing record is NOT the system
				// record.
				smsConfig.ProviderType = sms.ProviderTypeTwilio
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				smsConfig.TwilioAuthToken = form.TwilioAuthToken
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, realm, smsConfig, quotaLimit, quotaRemaining)
					return
				}
			} else {
				// There's no record or the existing record was the system config so we
				// want to create our own.
				smsConfig := &database.SMSConfig{
					RealmID:          realm.ID,
					ProviderType:     sms.ProviderTypeTwilio,
					TwilioAccountSid: form.TwilioAccountSid,
					TwilioAuthToken:  form.TwilioAuthToken,
					TwilioFromNumber: form.TwilioFromNumber,
				}

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, realm, smsConfig, quotaLimit, quotaRemaining)
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

func (c *Controller) renderSettings(ctx context.Context, w http.ResponseWriter, r *http.Request, realm *database.Realm, smsConfig *database.SMSConfig, quotaLimit, quotaRemaining uint64) {
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

	// Don't pass through the system config to the template - we don't want to
	// risk accidentally rendering its ID or values since the realm should never
	// see these values. However, we have to go lookup the actual SMS config
	// values if present so that if the user unchecks the form, they don't see
	// blank values if they were previously using their own SMS configs.
	if smsConfig.IsSystem {
		var tmpRealm database.Realm
		tmpRealm.ID = realm.ID
		tmpRealm.UseSystemSMSConfig = false

		var err error
		smsConfig, err = tmpRealm.SMSConfig(c.db)
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
	m["countries"] = database.Countries
	m["testTypes"] = map[string]database.TestType{
		"confirmed": database.TestTypeConfirmed,
		"likely":    database.TestTypeConfirmed | database.TestTypeLikely,
		"negative":  database.TestTypeConfirmed | database.TestTypeLikely | database.TestTypeNegative,
	}
	// Valid settings for pwd rotation.
	m["mfaGracePeriod"] = mfaGracePeriod
	m["passwordRotateDays"] = passwordRotationPeriodDays
	m["passwordWarnDays"] = passwordRotationWarningDays
	// Valid settings for code parameters.
	m["shortCodeLengths"] = shortCodeLengths
	m["shortCodeMinutes"] = shortCodeMinutes
	m["longCodeLengths"] = longCodeLengths
	m["longCodeHours"] = longCodeHours
	m["enxRedirectDomain"] = c.config.GetENXRedirectDomain()

	m["quotaLimit"] = quotaLimit
	m["quotaRemaining"] = quotaRemaining

	c.h.RenderHTML(w, "realmadmin/edit", m)
}
