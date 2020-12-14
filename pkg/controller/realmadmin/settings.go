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

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
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
		AllowBulkUpload       bool              `form:"allow_bulk"`
		RequireDate           bool              `form:"require_date"`
		CodeLength            uint              `form:"code_length"`
		CodeDurationMinutes   int64             `form:"code_duration"`
		LongCodeLength        uint              `form:"long_code_length"`
		LongCodeDurationHours int64             `form:"long_code_duration"`
		SMSTextTemplate       string            `form:"sms_text_template"`

		SMS                bool   `form:"sms"`
		UseSystemSMSConfig bool   `form:"use_system_sms_config"`
		SMSCountry         string `form:"sms_country"`
		SMSFromNumberID    uint   `form:"sms_from_number_id"`
		TwilioAccountSid   string `form:"twilio_account_sid"`
		TwilioAuthToken    string `form:"twilio_auth_token"`
		TwilioFromNumber   string `form:"twilio_from_number"`

		Email                bool   `form:"email"`
		UseSystemEmailConfig bool   `form:"use_system_email_config"`
		SMTPAccount          string `form:"smtp_account"`
		SMTPPassword         string `form:"smtp_password"`
		SMTPHost             string `form:"smtp_host"`
		SMTPPort             string `form:"smtp_port"`
		EmailInviteTemplate  string `form:"email_invite_template"`

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

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.SettingsRead) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm
		currentUser := membership.User

		var quotaLimit, quotaRemaining uint64
		if currentRealm.AbusePreventionEnabled {
			key, err := currentRealm.QuotaKey(c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			quotaLimit, quotaRemaining, err = c.limiter.Get(ctx, key)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		if r.Method == http.MethodGet {
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
			return
		}

		// If we got this far, we're about to update settings, upsert the higher
		// permission.
		if !membership.Can(rbac.SettingsWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to process form: %v", err)
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
			return
		}

		// General
		if form.General {
			currentRealm.Name = form.Name
			currentRealm.RegionCode = form.RegionCode
			currentRealm.WelcomeMessage = form.WelcomeMessage
		}

		// Codes
		if form.Codes {
			currentRealm.AllowedTestTypes = form.AllowedTestTypes
			currentRealm.RequireDate = form.RequireDate
			currentRealm.AllowBulkUpload = form.AllowBulkUpload
			currentRealm.SMSTextTemplate = form.SMSTextTemplate

			// These fields can only be set if ENX is disabled
			if !currentRealm.EnableENExpress {
				currentRealm.CodeLength = form.CodeLength
				currentRealm.CodeDuration.Duration = time.Duration(form.CodeDurationMinutes) * time.Minute
				currentRealm.LongCodeLength = form.LongCodeLength
				currentRealm.LongCodeDuration.Duration = time.Duration(form.LongCodeDurationHours) * time.Hour
			}
		}

		// SMS
		if form.SMS {
			currentRealm.UseSystemSMSConfig = form.UseSystemSMSConfig
			currentRealm.SMSCountry = form.SMSCountry
			currentRealm.SMSFromNumberID = form.SMSFromNumberID
		}

		// Email
		if form.Email {
			currentRealm.UseSystemEmailConfig = form.UseSystemEmailConfig
			currentRealm.EmailInviteTemplate = form.EmailInviteTemplate
		}

		// Security
		if form.Security {
			currentRealm.EmailVerifiedMode = database.AuthRequirement(form.EmailVerifiedMode)
			currentRealm.MFAMode = database.AuthRequirement(form.MFAMode)
			currentRealm.MFARequiredGracePeriod = database.FromDuration(time.Duration(form.MFARequiredGracePeriod) * 24 * time.Hour)
			currentRealm.PasswordRotationPeriodDays = form.PasswordRotationPeriodDays
			currentRealm.PasswordRotationWarningDays = form.PasswordRotationWarningDays

			allowedCIDRsAdminADPI, err := database.ToCIDRList(form.AllowedCIDRsAdminAPI)
			if err != nil {
				currentRealm.AddError("allowedCIDRsAdminAPI", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
				return
			}
			currentRealm.AllowedCIDRsAdminAPI = allowedCIDRsAdminADPI

			allowedCIDRsAPIServer, err := database.ToCIDRList(form.AllowedCIDRsAPIServer)
			if err != nil {
				currentRealm.AddError("allowedCIDRsAPIServer", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
				return
			}
			currentRealm.AllowedCIDRsAPIServer = allowedCIDRsAPIServer

			allowedCIDRsServer, err := database.ToCIDRList(form.AllowedCIDRsServer)
			if err != nil {
				currentRealm.AddError("allowedCIDRsServer", err.Error())
				flash.Error("Failed to update realm")
				c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
				return
			}
			currentRealm.AllowedCIDRsServer = allowedCIDRsServer
		}

		// Abuse prevention
		var abusePreventionJustEnabled bool
		if form.AbusePrevention {
			abusePreventionJustEnabled = !currentRealm.AbusePreventionEnabled && form.AbusePreventionEnabled

			currentRealm.AbusePreventionEnabled = form.AbusePreventionEnabled
			currentRealm.AbusePreventionLimitFactor = form.AbusePreventionLimitFactor
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
			key, err := currentRealm.QuotaKey(c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			limit := uint64(currentRealm.AbusePreventionEffectiveLimit())
			ttl := 7 * 24 * time.Hour
			if err := c.limiter.Set(ctx, key, limit, ttl); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// Save realm
		if err := c.db.SaveRealm(currentRealm, currentUser); err != nil {
			flash.Error("Failed to update realm: %v", err)
			c.renderSettings(ctx, w, r, currentRealm, nil, nil, quotaLimit, quotaRemaining)
			return
		}

		// SMS
		if form.SMS && !form.UseSystemSMSConfig {
			// Fetch the existing SMS config record, if one exists
			smsConfig, err := currentRealm.SMSConfig(c.db)
			if err != nil && !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			if smsConfig != nil && !smsConfig.IsSystem {
				// We have an existing record and the existing record is NOT the system
				// record.
				smsConfig.ProviderType = sms.ProviderTypeTwilio
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				if form.TwilioAuthToken != project.PasswordSentinel {
					smsConfig.TwilioAuthToken = form.TwilioAuthToken
				}
				smsConfig.TwilioFromNumber = form.TwilioFromNumber

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, currentRealm, smsConfig, nil, quotaLimit, quotaRemaining)
					return
				}
			} else {
				// There's no record or the existing record was the system config so we
				// want to create our own.
				smsConfig := &database.SMSConfig{
					RealmID:          currentRealm.ID,
					ProviderType:     sms.ProviderTypeTwilio,
					TwilioAccountSid: form.TwilioAccountSid,
					TwilioAuthToken:  form.TwilioAuthToken,
					TwilioFromNumber: form.TwilioFromNumber,
				}

				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, currentRealm, smsConfig, nil, quotaLimit, quotaRemaining)
					return
				}
			}
		}

		// Email
		if form.Email && !form.UseSystemEmailConfig {
			// Fetch the existing Email config record, if one exists
			emailConfig, err := currentRealm.EmailConfig(c.db)
			if err != nil && !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			if emailConfig != nil && !emailConfig.IsSystem {
				// We have an existing record and the existing record is NOT the system
				// record.
				emailConfig.ProviderType = email.ProviderTypeSMTP
				emailConfig.SMTPAccount = form.SMTPAccount
				if form.SMTPPassword != project.PasswordSentinel {
					emailConfig.SMTPPassword = form.SMTPPassword
				}
				emailConfig.SMTPHost = form.SMTPHost
				emailConfig.SMTPPort = form.SMTPPort

				if err := c.db.SaveEmailConfig(emailConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, currentRealm, nil, emailConfig, quotaLimit, quotaRemaining)
					return
				}
			} else {
				// There's no record or the existing record was the system config so we
				// want to create our own.
				emailConfig := &database.EmailConfig{
					RealmID:      currentRealm.ID,
					ProviderType: email.ProviderTypeSMTP,
					SMTPAccount:  form.SMTPAccount,
					SMTPPassword: form.SMTPPassword,
					SMTPHost:     form.SMTPHost,
					SMTPPort:     form.SMTPPort,
				}

				if err := c.db.SaveEmailConfig(emailConfig); err != nil {
					flash.Error("Failed to update realm: %v", err)
					c.renderSettings(ctx, w, r, currentRealm, nil, emailConfig, quotaLimit, quotaRemaining)
					return
				}
			}
		}

		// Process temporary abuse prevention bursts
		if burst := form.AbusePreventionBurst; burst > 0 {
			key, err := currentRealm.QuotaKey(c.config.RateLimit.HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			if err := c.limiter.Burst(ctx, key, burst); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			msg := fmt.Sprintf("added %d quota capacity", burst)
			audit := database.BuildAuditEntry(currentUser, msg, currentRealm, currentRealm.ID)
			if err := c.db.SaveAuditEntry(audit); err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}

			flash.Alert("Successfully added %d to realm quota!", burst)
		}

		flash.Alert("Successfully updated realm settings!")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}

func (c *Controller) renderSettings(
	ctx context.Context, w http.ResponseWriter, r *http.Request, currentRealm *database.Realm,
	smsConfig *database.SMSConfig, emailConfig *database.EmailConfig, quotaLimit, quotaRemaining uint64) {
	if smsConfig == nil {
		var err error
		smsConfig, err = currentRealm.SMSConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			smsConfig = new(database.SMSConfig)
		}
	}

	if emailConfig == nil {
		var err error
		emailConfig, err = currentRealm.EmailConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			emailConfig = &database.EmailConfig{SMTPPort: "587"}
		}
	}

	// Look up the sms from numbers.
	smsFromNumbers, err := c.db.SMSFromNumbers()
	if err != nil {
		controller.InternalError(w, r, c.h, err)
		return
	}

	// Don't pass through the system config to the template - we don't want to
	// risk accidentally rendering its ID or values since the realm should never
	// see these values. However, we have to go lookup the actual SMS config
	// values if present so that if the user unchecks the form, they don't see
	// blank values if they were previously using their own SMS configs.
	if smsConfig.IsSystem {
		var tmpRealm database.Realm
		tmpRealm.ID = currentRealm.ID
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

	if emailConfig.IsSystem {
		var tmpRealm database.Realm
		tmpRealm.ID = currentRealm.ID
		tmpRealm.UseSystemEmailConfig = false

		var err error
		emailConfig, err = tmpRealm.EmailConfig(c.db)
		if err != nil {
			if !database.IsNotFound(err) {
				controller.InternalError(w, r, c.h, err)
				return
			}
			emailConfig = new(database.EmailConfig)
		}
	}

	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm settings")
	m["realm"] = currentRealm
	m["smsConfig"] = smsConfig
	m["smsFromNumbers"] = smsFromNumbers
	m["emailConfig"] = emailConfig
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

	m["maxSMSTemplate"] = database.SMSTemplateMaxLength

	m["quotaLimit"] = quotaLimit
	m["quotaRemaining"] = quotaRemaining

	c.h.RenderHTML(w, "realmadmin/edit", m)
}
