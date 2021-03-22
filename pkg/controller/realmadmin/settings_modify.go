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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/jinzhu/gorm/dialects/postgres"
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

const (
	labelPrefix    = "sms_text_label_"
	templatePrefix = "sms_text_template_"
)

func init() {
	for i := 5; i <= 60; i++ {
		shortCodeMinutes = append(shortCodeMinutes, i)
	}
	for i := 1; i <= 24; i++ {
		longCodeHours = append(longCodeHours, i)
	}
}

type formData struct {
	General        bool   `form:"general"`
	Name           string `form:"name"`
	RegionCode     string `form:"region_code"`
	WelcomeMessage string `form:"welcome_message"`

	AllowKeyServerStats       bool   `form:"allow_key_server_stats"`
	KeyServerURLOverride      string `form:"key_server_url"`
	KeyServerAudienceOverride string `form:"key_server_audience"`

	Codes                 bool              `form:"codes"`
	AllowedTestTypes      database.TestType `form:"allowed_test_types"`
	AllowUserReport       bool              `form:"allow_user_report"`
	AllowBulkUpload       bool              `form:"allow_bulk"`
	RequireDate           bool              `form:"require_date"`
	CodeLength            uint              `form:"code_length"`
	CodeDurationMinutes   int64             `form:"code_duration"`
	LongCodeLength        uint              `form:"long_code_length"`
	LongCodeDurationHours int64             `form:"long_code_duration"`

	SMS                       bool               `form:"sms"`
	UseSystemSMSConfig        bool               `form:"use_system_sms_config"`
	SMSCountry                string             `form:"sms_country"`
	SMSFromNumberID           uint               `form:"sms_from_number_id"`
	TwilioAccountSid          string             `form:"twilio_account_sid"`
	TwilioAuthToken           string             `form:"twilio_auth_token"`
	TwilioFromNumber          string             `form:"twilio_from_number"`
	SMSTextTemplate           string             `form:"-"`
	SMSTextAlternateTemplates map[string]*string `form:"-"`
	SMSTextUserReportAppend   string             `form:"sms_text_user_report_append"`

	Email                      bool   `form:"email"`
	UseSystemEmailConfig       bool   `form:"use_system_email_config"`
	SMTPAccount                string `form:"smtp_account"`
	SMTPPassword               string `form:"smtp_password"`
	SMTPHost                   string `form:"smtp_host"`
	SMTPPort                   string `form:"smtp_port"`
	EmailInviteTemplate        string `form:"email_invite_template"`
	EmailPasswordResetTemplate string `form:"password_reset_template"`
	EmailVerifyTemplate        string `form:"email_verify_template"`

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

func (c *Controller) HandleSettings() http.Handler {
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

		statsConfig, err := c.db.GetKeyServerStats(currentRealm.ID)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		smsConfig, err := currentRealm.SMSConfig(c.db)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		emailConfig, err := currentRealm.EmailConfig(c.db)
		if err != nil && !database.IsNotFound(err) {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if r.Method == http.MethodGet {
			c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
			return
		}

		// If we got this far, we're about to update settings, upsert the higher
		// permission.
		if !membership.Can(rbac.SettingsWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		var form formData
		if err := controller.BindForm(w, r, &form); err != nil {
			currentRealm.AddError("", err.Error())
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
			return
		}

		// General
		if form.General {
			currentRealm.Name = form.Name
			currentRealm.RegionCode = form.RegionCode
			currentRealm.WelcomeMessage = form.WelcomeMessage

			if form.AllowKeyServerStats {
				if statsConfig == nil {
					// There's no record or the existing record was the system config so we
					// want to create our own.
					statsConfig = &database.KeyServerStats{
						RealmID:                   currentRealm.ID,
						KeyServerURLOverride:      form.KeyServerURLOverride,
						KeyServerAudienceOverride: form.KeyServerAudienceOverride,
					}
				} else {
					statsConfig.KeyServerURLOverride = form.KeyServerURLOverride
					statsConfig.KeyServerAudienceOverride = form.KeyServerAudienceOverride
				}

				if err := c.db.SaveKeyServerStats(statsConfig); err != nil {
					if database.IsValidationError(err) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
						return
					}

					controller.InternalError(w, r, c.h, err)
					return
				}
			} else if statsConfig != nil {
				if err := c.db.DeleteKeyServerStats(currentRealm.ID); err != nil {
					controller.InternalError(w, r, c.h, err)
					return
				}
			}
		}

		// Codes
		if form.Codes {
			currentRealm.AllowedTestTypes = form.AllowedTestTypes
			currentRealm.RequireDate = form.RequireDate
			currentRealm.AllowBulkUpload = form.AllowBulkUpload
			if c.config.Features.EnableUserReport && form.AllowUserReport {
				currentRealm.AddUserReportToAllowedTestTypes()
			}

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
			parseSMSTextTemplates(r, &form)
			currentRealm.UseSystemSMSConfig = form.UseSystemSMSConfig
			currentRealm.SMSCountry = form.SMSCountry
			currentRealm.SMSFromNumberID = form.SMSFromNumberID
			currentRealm.SMSTextTemplate = form.SMSTextTemplate
			currentRealm.SMSTextAlternateTemplates = postgres.Hstore(form.SMSTextAlternateTemplates)
		}

		// Email
		if form.Email {
			currentRealm.UseSystemEmailConfig = form.UseSystemEmailConfig
			currentRealm.EmailInviteTemplate = form.EmailInviteTemplate
			currentRealm.EmailPasswordResetTemplate = form.EmailPasswordResetTemplate
			currentRealm.EmailVerifyTemplate = form.EmailVerifyTemplate
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
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
				return
			}
			currentRealm.AllowedCIDRsAdminAPI = allowedCIDRsAdminADPI

			allowedCIDRsAPIServer, err := database.ToCIDRList(form.AllowedCIDRsAPIServer)
			if err != nil {
				currentRealm.AddError("allowedCIDRsAPIServer", err.Error())
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
				return
			}
			currentRealm.AllowedCIDRsAPIServer = allowedCIDRsAPIServer

			allowedCIDRsServer, err := database.ToCIDRList(form.AllowedCIDRsServer)
			if err != nil {
				currentRealm.AddError("allowedCIDRsServer", err.Error())
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
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
			if database.IsValidationError(err) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// SMS
		if form.SMS && !form.UseSystemSMSConfig {
			if smsConfig != nil && !smsConfig.IsSystem {
				// We have an existing record and the existing record is NOT the system
				// record.
				smsConfig.ProviderType = sms.ProviderTypeTwilio
				smsConfig.TwilioAccountSid = form.TwilioAccountSid
				if form.TwilioAuthToken != project.PasswordSentinel {
					smsConfig.TwilioAuthToken = form.TwilioAuthToken
				}
				smsConfig.TwilioFromNumber = form.TwilioFromNumber
			} else {
				// There's no record or the existing record was the system config so we
				// want to create our own.
				smsConfig = &database.SMSConfig{
					RealmID:          currentRealm.ID,
					ProviderType:     sms.ProviderTypeTwilio,
					TwilioAccountSid: form.TwilioAccountSid,
					TwilioAuthToken:  form.TwilioAuthToken,
					TwilioFromNumber: form.TwilioFromNumber,
				}
			}

			if !smsConfig.IsSystem {
				if err := c.db.SaveSMSConfig(smsConfig); err != nil {
					if database.IsValidationError(err) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
						return
					}

					controller.InternalError(w, r, c.h, err)
					return
				}

				flash.Warning("It can take up to 5 minutes for the new SMS configuration to be fully propagated.")
			}
		}

		// Email
		if form.Email && !form.UseSystemEmailConfig {
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
			} else {
				// There's no record or the existing record was the system config so we
				// want to create our own.
				emailConfig = &database.EmailConfig{
					RealmID:      currentRealm.ID,
					ProviderType: email.ProviderTypeSMTP,
					SMTPAccount:  form.SMTPAccount,
					SMTPPassword: form.SMTPPassword,
					SMTPHost:     form.SMTPHost,
					SMTPPort:     form.SMTPPort,
				}
			}

			if !emailConfig.IsSystem {
				if err := c.db.SaveEmailConfig(emailConfig); err != nil {
					if database.IsValidationError(err) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						c.renderSettings(ctx, w, r, currentRealm, smsConfig, emailConfig, statsConfig, quotaLimit, quotaRemaining)
						return
					}

					controller.InternalError(w, r, c.h, err)
					return
				}

				flash.Warning("It can take up to 5 minutes for the new email configuration to be fully propagated.")
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

			flash.Alert("Successfully added %d to realm quota", burst)
		}

		flash.Alert("Successfully updated realm settings")
		http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
	})
}

func parseSMSTextTemplates(r *http.Request, form *formData) {
	// Associate by index
	templates := map[string]*TemplateData{}
	for k, v := range r.PostForm {
		s := v[0]
		if strings.HasPrefix(k, labelPrefix) {
			i := k[len(labelPrefix):]
			if t, has := templates[i]; has {
				t.Label = s
			} else {
				templates[i] = &TemplateData{Label: s}
			}
		}
		if strings.HasPrefix(k, templatePrefix) {
			i := k[len(templatePrefix):]
			if t, has := templates[i]; has {
				t.Value = s
			} else {
				templates[i] = &TemplateData{Value: s}
			}
		}
	}
	// Copy paired label/values to the alt templates
	if len(templates) > 0 {
		form.SMSTextAlternateTemplates = map[string]*string{}
		for k, v := range templates {
			if k == "0" {
				form.SMSTextTemplate = v.Value
			} else {
				s := v.Value
				form.SMSTextAlternateTemplates[v.Label] = &s
			}
		}
	}
}
