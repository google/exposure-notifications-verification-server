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

package database

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"github.com/jinzhu/gorm"
	"github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
	"github.com/russross/blackfriday/v2"
)

// TestType is a test type in the database.
type TestType int16

const (
	_ TestType = 1 << iota
	TestTypeConfirmed
	TestTypeLikely
	TestTypeNegative
	TestTypeUserReport
)

func (t TestType) Display() string {
	var types []string

	if t&TestTypeConfirmed != 0 {
		types = append(types, "confirmed")
	}

	if t&TestTypeLikely != 0 {
		types = append(types, "likely")
	}

	if t&TestTypeNegative != 0 {
		types = append(types, "negative")
	}

	if t&TestTypeUserReport != 0 {
		types = append(types, "user-report")
	}

	return strings.Join(types, ", ")
}

// AuthRequirement represents authentication requirements for the realm
type AuthRequirement int16

const (
	// MFAOptionalPrompt will prompt users for MFA on login.
	MFAOptionalPrompt AuthRequirement = iota
	// MFARequired will not allow users to proceed without MFA on their account.
	MFARequired
	// MFAOptional will not prompt users to enable MFA.
	MFAOptional
)

func (r AuthRequirement) String() string {
	switch r {
	case MFAOptionalPrompt:
		return "prompt"
	case MFARequired:
		return "required"
	case MFAOptional:
		return "optional"
	}
	return ""
}

var (
	ErrNoSigningKeyManagement = errors.New("no signing key management")
	ErrBadDateRange           = errors.New("bad date range")

	ENXRedirectDomain = os.Getenv("ENX_REDIRECT_DOMAIN")

	colorRegex = regexp.MustCompile(`\A#[0-9a-f]{6}\z`)
)

const (
	DefaultShortCodeLength            = 8
	DefaultShortCodeExpirationMinutes = 15
	DefaultLongCodeLength             = 16
	DefaultLongCodeExpirationHours    = 24
	DefaultMaxShortCodeMinutes        = 60
	maxLongCodeDuration               = 24 * time.Hour
	DefaultSMSRegion                  = "us"
	DefaultLanguage                   = "en"

	SMSRegion        = "[region]"
	SMSCode          = "[code]"
	SMSExpires       = "[expires]"
	SMSLongCode      = "[longcode]"
	SMSLongExpires   = "[longexpires]"
	SMSENExpressLink = "[enslink]"

	SMSTemplateMaxLength    = 800
	SMSTemplateExpansionMax = 918

	DefaultTemplateLabel      = "Default SMS template"
	DefaultSMSTextTemplate    = "This is your Exposure Notifications Verification code: [longcode] Expires in [longexpires] hours"
	DefaultENXSMSTextTemplate = "Your Exposure Notifications verification link: [enslink] Expires in [longexpires] hours (click for mobile device only)"
	UserReportTemplateLabel   = "User Report"
	UserReportDefaultText     = "Your requested Exposure Notifications code: [code] expires in [expires] minutes. If you did not request this code, please ignore this message."
	UserReportDefaultENXText  = "Your requested Exposure Notifications link: [enslink] expires in [expires] minutes. If you did not request this code, please ignore this message."

	EmailInviteLink        = "[invitelink]"
	EmailPasswordResetLink = "[passwordresetlink]"
	EmailVerifyLink        = "[verifylink]"
	RealmName              = "[realmname]"

	// MaxPageSize is the maximum allowed page size for a list query.
	MaxPageSize = 1000
)

var _ Auditable = (*Realm)(nil)

// Realm represents a tenant in the system. Typically this corresponds to a
// geography or a public health authority scope.
// This is used to manage user logins.
type Realm struct {
	gorm.Model
	Errorable

	// Name is the name of the realm.
	Name string `gorm:"type:varchar(200);unique_index;"`

	// RegionCode is both a display attribute and required field for ENX. To
	// handle NULL and uniqueness, the field is converted from it's ptr type to a
	// concrete type in callbacks. Do not modify RegionCodePtr directly.
	RegionCode    string  `gorm:"-"`
	RegionCodePtr *string `gorm:"column:region_code; type:varchar(10);"`

	// WelcomeMessage is arbitrary realm-defined data to display to users after
	// selecting this realm. If empty, nothing is displayed. The format is
	// markdown. Do not modify WelcomeMessagePtr directly.
	WelcomeMessage    string  `gorm:"-"`
	WelcomeMessagePtr *string `gorm:"column:welcome_message; type:text;"`

	// AgencyBackgroundColor, AgencyImage, DefaultLocale are synced from the Google
	// ENX-Express sync source
	AgencyBackgroundColor     string  `gorm:"-"`
	AgencyBackgroundColorPtr  *string `gorm:"column:agency_background_color; type:text;"`
	AgencyImage               string  `gorm:"-"`
	AgencyImagePtr            *string `gorm:"column:agency_image; type:text;"`
	DefaultLocale             string  `gorm:"-"`
	DefaultLocalePtr          *string `gorm:"column:default_locale; type:text;"`
	UserReportLearnMoreURL    string  `gorm:"-"`
	UserReportLearnMoreURLPtr *string `gorm:"column:user_report_learn_more_url; type:text;"`

	// UserReportWebhookURL and UserReportWebhookSecret are used as callbacks for
	// user reports.
	UserReportWebhookURL                   string  `gorm:"-"`
	UserReportWebhookURLPtr                *string `gorm:"column:user_report_webhook_url; type:text;"`
	UserReportWebhookSecret                string  `gorm:"-" json:"-"`
	UserReportWebhookSecretPtr             *string `gorm:"column:user_report_webhook_secret; type:text;" json:"-"`
	UserReportWebhookSecretPlaintextCache  string  `gorm:"-"`
	UserReportWebhookSecretCiphertextCache string  `gorm:"-"`

	// AllowBulkUpload allows users to issue codes from a batch file of test results.
	AllowBulkUpload bool `gorm:"type:boolean; not null; default:false;"`

	// Code configuration
	CodeLength       uint            `gorm:"type:smallint; not null; default: 8;"`
	CodeDuration     DurationSeconds `gorm:"type:bigint; not null; default: 900;"` // default 15m (in seconds)
	LongCodeLength   uint            `gorm:"type:smallint; not null; default: 16;"`
	LongCodeDuration DurationSeconds `gorm:"type:bigint; not null; default: 86400;"` // default 24h

	// ShortCodeMaxMinutes can only be set by system admins and allows for a
	// realm to have a higher max short code duration
	ShortCodeMaxMinutes uint `gorm:"column:short_code_max_minutes; type:smallint; not null; default: 60;"`
	// ENXCodeExpirationConfigurable can only be set by system admins and allows
	// for an ENX realm to change the short code expiration time (normally fixed)
	ENXCodeExpirationConfigurable bool `gorm:"column:enx_code_expiration_configurable; type:bool; not null; default: false;"`

	// SMS configuration
	SMSTextTemplate           string          `gorm:"type:text; not null; default: 'This is your Exposure Notifications Verification code: [longcode] Expires in [longexpires] hours';"`
	SMSTextAlternateTemplates postgres.Hstore `gorm:"column:alternate_sms_templates; type:hstore;"`

	// SMSCountry is an optional field to hint the default phone picker country
	// code.
	SMSCountry    string  `gorm:"-"`
	SMSCountryPtr *string `gorm:"column:sms_country; type:varchar(5);"`

	// CanUseSystemSMSConfig is configured by system administrators to share the
	// system SMS config with this realm. Note that the system SMS config could be
	// empty and a local SMS config is preferred over the system value.
	CanUseSystemSMSConfig bool `gorm:"column:can_use_system_sms_config; type:bool; not null; default:false;"`

	// UseSystemSMSConfig is a realm-level configuration that lets a realm opt-out
	// of sending SMS messages using the system-provided SMS configuration.
	// Without this, a realm would always fallback to the system-level SMS
	// configuration, making it impossible to opt out of text message sending.
	UseSystemSMSConfig bool `gorm:"column:use_system_sms_config; type:bool; not null; default:false;"`

	// SMSFromNumberID is a realm-level configuration that only applies when using
	// the system SMS configuration. It determines which of the system SMS numbers
	// to use as the sender when sending text messages.
	SMSFromNumberID    uint  `gorm:"-"`
	SMSFromNumberIDPtr *uint `gorm:"column:sms_from_number_id; type:integer;"`

	// UseAuthenticatedSMS indicates if this realm wants to sign text messages that are sent
	// containing verification codes.
	UseAuthenticatedSMS bool `gorm:"column:use_authenticated_sms; type:bool; not null; default:false;"`

	// AllowGeneratedSMS indicates if this realm can request generated SMS
	// messages via the API. If enabled, callers can request a fully-compiled and
	// signed (if Authenticated SMS is enabled) SMS message to be returned when
	// calling the issue API.
	AllowGeneratedSMS bool `gorm:"column:allow_generated_sms; type:bool; not null; default:false;"`

	// EmailInviteTemplate is the template for inviting new users.
	EmailInviteTemplate string `gorm:"type:text;"`

	// EmailPasswordResetTemplate is the template for resetting password.
	EmailPasswordResetTemplate string `gorm:"type:text;"`

	// EmailVerifyTemplate is the template used for email verification.
	EmailVerifyTemplate string `gorm:"type:text;"`

	// CanUseSystemEmailConfig is configured by system administrators to share the
	// system email config with this realm. Note that the system email config could be
	// empty and a local email config is preferred over the system value.
	CanUseSystemEmailConfig bool `gorm:"column:can_use_system_email_config; type:bool; not null; default:false;"`

	// UseSystemEmailConfig is a realm-level configuration that lets a realm opt-out
	// of sending email messages using the system-provided email configuration.
	// Without this, a realm would always fallback to the system-level email
	// configuration, making it impossible to opt out of text message sending.
	UseSystemEmailConfig bool `gorm:"column:use_system_email_config; type:bool; not null; default:false;"`

	// MFAMode represents the mode for Multi-Factor-Authorization requirements for the realm.
	MFAMode AuthRequirement `gorm:"type:smallint; not null; default: 0;"`

	// MFARequiredGracePeriod defines how long after creation a user may skip adding
	// a second auth factor before the server requires it.
	MFARequiredGracePeriod DurationSeconds `gorm:"type:bigint; not null; default: 0;"`

	// EmailVerifiedMode represents the mode for email verification requirements for the realm.
	EmailVerifiedMode AuthRequirement `gorm:"type:smallint; not null; default: 0;"`

	// PasswordRotationPeriodDays is the number of days before the user must
	// rotate their password.
	PasswordRotationPeriodDays uint `gorm:"type:smallint; not null; default: 0;"`

	// PasswordRotationWarningDays is the number of days before Password expiry
	// that the user should receive a warning.
	PasswordRotationWarningDays uint `gorm:"type:smallint; not null; default: 0;"`

	// AllowedCIDRs is the list of allowed IPs to the various services.
	AllowedCIDRsAdminAPI  pq.StringArray `gorm:"column:allowed_cidrs_adminapi; type:varchar(50)[];"`
	AllowedCIDRsAPIServer pq.StringArray `gorm:"column:allowed_cidrs_apiserver; type:varchar(50)[];"`
	AllowedCIDRsServer    pq.StringArray `gorm:"column:allowed_cidrs_server; type:varchar(50)[];"`

	// AllowedTestTypes is the type of tests that this realm permits. The default
	// value is to allow all test types.
	AllowedTestTypes TestType `gorm:"type:smallint; not null; default: 14;"`

	// AllowUserReportWebView - if enabled, will use the user report web view
	// on the redirect server for this realm. If disabled, it will 404.
	AllowUserReportWebView bool `gorm:"column:allow_user_report_web_view; type:bool; not null; default:false"`

	// AllowAdminUserReport - is the adminapi:/api/issue allowed to use the user-report
	// test type if enabled on the realm.
	AllowAdminUserReport bool `gorm:"column:allow_admin_user_report; type:bool; not null; default:false"`

	// RequireDate requires that verifications on this realm require a test or
	// symptom date (either). The default behavior is to not require a date.
	RequireDate bool `gorm:"type:boolean; not null; default:false;"`

	// Signing Key Settings
	UseRealmCertificateKey   bool            `gorm:"type:boolean; default: false;"`
	CertificateIssuer        string          `gorm:"type:varchar(150); default: '';"`
	CertificateAudience      string          `gorm:"type:varchar(150); default: '';"`
	CertificateDuration      DurationSeconds `gorm:"type:bigint; default: 900;"` // 15m
	AutoRotateCertificateKey bool            `gorm:"type:boolean; default: false;"`

	// EN Express
	EnableENExpress bool `gorm:"type:boolean; default: false;"`

	// AbusePreventionEnabled determines if abuse protection is enabled.
	AbusePreventionEnabled bool `gorm:"type:boolean; not null; default:false;"`

	// AbusePreventionLimit is the configured daily limit for the realm. This value is populated
	// by the nightly aggregation job and is based on a statistical model from
	// historical code issuance data.
	AbusePreventionLimit uint `gorm:"type:integer; not null; default:10;"`

	// AbusePreventionLimitFactor is the factor against the predicted model for the day which
	// determines the total number of codes that can be issued for the realm on
	// the day. For example, if the predicted value was 50 and this value was 1.5,
	// the realm could generate 75 codes today before triggering abuse prevention.
	// Similarly, if this value was 0.5, the realm could only generate 25 codes
	// before triggering abuse protections.
	AbusePreventionLimitFactor float32 `gorm:"type:numeric(6, 3); not null; default:1.0;"`

	// LastCodesClaimedRatio is the percentage of codes claimed (out of all codes
	// issued) for the most recent completely full UTC day. CodesClaimedRatioMean and
	// CodesClaimedRatioStddev represent the mean and standard deviation for the
	// previous N days of statistics (see modeler for exact numbers).
	//
	// The *Tokens fields are identical, except the represent the ratio between
	// codes claimed and tokens claimed (aka app-side redemption). A high
	// deviation here likely indicates some kind of application-level issue.
	//
	// These fields are set by the modeler.
	LastCodesClaimedRatio   float64 `gorm:"column:last_codes_claimed_ratio; type:numeric(10,8); not null; default:0.0;"`
	CodesClaimedRatioMean   float64 `gorm:"column:codes_claimed_ratio_mean; type:numeric(10,8); not null; default:0.0;"`
	CodesClaimedRatioStddev float64 `gorm:"column:codes_claimed_ratio_stddev; type:numeric(10,8); not null; default:0.0;"`

	// Relations to items that belong to a realm.
	Codes  []*VerificationCode `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false;"`
	Tokens []*Token            `gorm:"PRELOAD:false; SAVE_ASSOCIATIONS:false; ASSOCIATION_AUTOUPDATE:false, ASSOCIATION_SAVE_REFERENCE:false;"`

	// enxRedirectDomainOverride is a value to override the global ENX redirect on
	// a per-realm basis, primarily for testing.
	enxRedirectDomainOverride string
}

// NewRealmWithDefaults initializes a new Realm with the default settings populated,
// and the provided name. It does NOT save the Realm to the database.
func NewRealmWithDefaults(name string) *Realm {
	return &Realm{
		Name:                name,
		CodeLength:          DefaultShortCodeLength,
		CodeDuration:        FromDuration(DefaultShortCodeExpirationMinutes * time.Minute),
		LongCodeLength:      DefaultLongCodeLength,
		LongCodeDuration:    FromDuration(DefaultLongCodeExpirationHours * time.Hour),
		ShortCodeMaxMinutes: DefaultMaxShortCodeMinutes,
		SMSTextTemplate:     DefaultSMSTextTemplate,
		SMSCountry:          DefaultSMSRegion,
		AllowedTestTypes:    TestTypeConfirmed | TestTypeLikely | TestTypeNegative,
		CertificateDuration: FromDuration(15 * time.Minute),
		RequireDate:         true, // Having dates is really important to risk scoring, encourage this by default true.
		DefaultLocale:       DefaultLanguage,
	}
}

// DefaultSMSTextTemplate returns correct default SMS Template for the realm.
func (r *Realm) DefaultSMSTextTemplate() string {
	if r.EnableENExpress {
		return DefaultENXSMSTextTemplate
	}
	return DefaultSMSTextTemplate
}

// DefaultUserReportSMSTextTemplate returns the correct default User Report
// template for the realm.
func (r *Realm) DefaultUserReportSMSTextTemplate() string {
	if r.EnableENExpress {
		return UserReportDefaultENXText
	}
	return UserReportDefaultText
}

// ResetSMSTextTemplates will update all of the templates based on the
// ENX Redirect setting
func (r *Realm) ResetSMSTextTemplates() {
	r.SMSTextTemplate = r.DefaultSMSTextTemplate()
	// If there is a UserReport - upgrade that message as well
	if _, ok := r.SMSTextAlternateTemplates[UserReportTemplateLabel]; ok {
		m := r.DefaultUserReportSMSTextTemplate()
		r.SMSTextAlternateTemplates[UserReportTemplateLabel] = &m
	}
	// Upgrade other messages
	for k, v := range r.SMSTextAlternateTemplates {
		if !strings.Contains(*v, SMSENExpressLink) {
			m := r.DefaultSMSTextTemplate()
			r.SMSTextAlternateTemplates[k] = &m
		}
	}
}

// SMSTemplateMaxLength returns database.SMSTemplateMaxLength.
// Convenance for utilizing in HTML templates.
func (r *Realm) SMSTemplateMaxLength() int {
	return SMSTemplateMaxLength
}

// SMSTemplateExpansionMax returns database.SMSTemplateExpansionMax.
// Convenance for utilizing in HTML templates.
func (r *Realm) SMSTemplateExpansionMax() int {
	return SMSTemplateExpansionMax
}

// E2ERealm gets the end-to-end realm. The end-to-end realm is defined as the
// realm that has a region_code beginning with E2E-* or a name beginning with
// e2e-test-*.
//
// If no e2e realm is defined, it returns NotFound.
func (db *Database) E2ERealm() (*Realm, error) {
	var realm Realm
	if err := db.db.
		Model(&Realm{}).
		Where("region_code ILIKE 'E2E-%' OR name ILIKE 'e2e-test-%'").
		Order("created_at ASC").
		First(&realm).
		Error; err != nil {
		return nil, fmt.Errorf("failed to find e2e realm: %w", err)
	}
	return &realm, nil
}

// AllowsUserReport returns true if this realm has enabled user initiated
// test reporting.
func (r *Realm) AllowsUserReport() bool {
	return r.AllowedTestTypes&TestTypeUserReport != 0
}

// AddUserReportToAllowedTestTypes adds the TestTypeUserReport to this realm.
// This does not save the realm to the database.
func (r *Realm) AddUserReportToAllowedTestTypes() {
	r.AllowedTestTypes = r.AllowedTestTypes | TestTypeUserReport
}

// AfterFind runs after a realm is found.
func (r *Realm) AfterFind(tx *gorm.DB) error {
	r.RegionCode = stringValue(r.RegionCodePtr)
	r.WelcomeMessage = stringValue(r.WelcomeMessagePtr)
	r.SMSCountry = stringValue(r.SMSCountryPtr)
	r.SMSFromNumberID = uintValue(r.SMSFromNumberIDPtr)
	r.AgencyBackgroundColor = stringValue(r.AgencyBackgroundColorPtr)
	r.AgencyImage = stringValue(r.AgencyImagePtr)
	r.UserReportWebhookURL = stringValue(r.UserReportWebhookURLPtr)
	r.UserReportWebhookSecret = stringValue(r.UserReportWebhookSecretPtr)
	r.DefaultLocale = stringValue(r.DefaultLocalePtr)
	if r.DefaultLocale == "" {
		r.DefaultLocale = DefaultLanguage
	}
	r.UserReportLearnMoreURL = stringValue(r.UserReportLearnMoreURLPtr)

	return nil
}

// BeforeSave runs validations. If there are errors, the save fails.
func (r *Realm) BeforeSave(tx *gorm.DB) error {
	r.Name = project.TrimSpace(r.Name)
	if r.Name == "" {
		r.AddError("name", "cannot be blank")
	}

	r.RegionCode = strings.ToUpper(project.TrimSpace(r.RegionCode))
	if len(r.RegionCode) > 10 {
		r.AddError("regionCode", "cannot be more than 10 characters")
	}
	r.RegionCodePtr = stringPtr(r.RegionCode)

	r.WelcomeMessage = project.TrimSpace(r.WelcomeMessage)
	r.WelcomeMessagePtr = stringPtr(r.WelcomeMessage)

	if c := r.AgencyBackgroundColor; c != "" && !colorRegex.MatchString(c) {
		r.AddError("agencyBackgroundColor", "is not a valid hex color string")
	}
	r.AgencyBackgroundColorPtr = stringPtr(r.AgencyBackgroundColor)
	r.AgencyImagePtr = stringPtr(r.AgencyImage)

	r.UserReportWebhookSecret = project.TrimSpace(r.UserReportWebhookSecret)
	r.UserReportWebhookSecretPtr = stringPtr(r.UserReportWebhookSecret)

	r.UserReportWebhookURL = project.TrimSpace(r.UserReportWebhookURL)
	if v := r.UserReportWebhookURL; v != "" {
		u, err := url.Parse(v)
		if err != nil {
			r.AddError("userReportWebhookURL", "is not a valid URL")
		}
		if u.Scheme != "https" {
			r.AddError("userReportWebhookURL", "must begin with https://")
		}

		// A webhook secret is required if a URL was provided.
		if want := 12; len(r.UserReportWebhookSecret) < want {
			r.AddError("userReportWebhookSecret", fmt.Sprintf("must be at least %d characters", want))
		}
	}
	r.UserReportWebhookURLPtr = stringPtr(r.UserReportWebhookURL)

	r.DefaultLocalePtr = stringPtr(r.DefaultLocale)
	r.UserReportLearnMoreURLPtr = stringPtr(r.UserReportLearnMoreURL)

	if r.UseSystemSMSConfig && !r.CanUseSystemSMSConfig {
		r.AddError("useSystemSMSConfig", "is not allowed on this realm")
	}

	if r.UseSystemSMSConfig && r.SMSFromNumberID == 0 {
		r.AddError("smsFromNumber", "is required to use the system config")
	}

	r.SMSCountryPtr = stringPtr(r.SMSCountry)

	r.SMSFromNumberIDPtr = uintPtr(r.SMSFromNumberID)

	if r.EnableENExpress {
		if r.RegionCode == "" {
			r.AddError("regionCode", "cannot be blank when using EN Express")
		}
	}

	if r.PasswordRotationWarningDays > r.PasswordRotationPeriodDays {
		r.AddError("passwordWarn", "may not be longer than password rotation period")
	}

	// TODO(mikehelmick) - make these configurable. There isn't currently a good way
	// to thread config to this point though.
	if r.ShortCodeMaxMinutes < 60 || r.ShortCodeMaxMinutes > 120 {
		r.AddError("shortCodeMaxMinutes", "must be >= 60 and <= 120")
	}

	if r.CodeLength < 6 {
		r.AddError("codeLength", "must be at least 6")
	}

	// Validation of the max code duration is dependent on overrides.
	realmMaxCodeDuration := time.Minute * time.Duration(r.ShortCodeMaxMinutes)
	if r.CodeDuration.Duration > realmMaxCodeDuration {
		r.AddError("codeDuration", fmt.Sprintf("must be no more than %v minutes", r.ShortCodeMaxMinutes))
	}

	if r.LongCodeLength < 12 {
		r.AddError("longCodeLength", "must be at least 12")
	}
	if r.LongCodeDuration.Duration > maxLongCodeDuration {
		r.AddError("longCodeDuration", "must be no more than 24 hours")
	}

	r.validateSMSTemplate(DefaultTemplateLabel, r.SMSTextTemplate)

	// See if the user report template needs to be added into the mix.
	if r.SMSTextAlternateTemplates == nil && r.AllowsUserReport() {
		r.SMSTextAlternateTemplates = make(postgres.Hstore)
	}
	if r.AllowsUserReport() {
		if _, ok := r.SMSTextAlternateTemplates[UserReportTemplateLabel]; !ok {
			newText := UserReportDefaultText
			if r.EnableENExpress {
				newText = UserReportDefaultENXText
			}
			r.SMSTextAlternateTemplates[UserReportTemplateLabel] = &newText
		}
	} else {
		if r.AllowUserReportWebView {
			r.AddError("allowUserReportWebView", "cannot be enabled unless user report is enabled")
		}
		if r.AllowAdminUserReport {
			r.AddError("allowAdminUserReport", "cannot be enabled unless user report is enabled")
		}
	}

	if r.SMSTextAlternateTemplates != nil {
		for l, t := range r.SMSTextAlternateTemplates {
			if t == nil || *t == "" {
				r.AddError("smsTextTemplate", fmt.Sprintf("no template for label %s", l))
				r.AddError(l, fmt.Sprintf("no template for label %s", l))
				continue
			}
			if l == "" {
				r.AddError("smsTextTemplate", fmt.Sprintf("no label for template %s", *t))
				continue
			}
			if l == DefaultTemplateLabel {
				r.AddError("smsTextTemplate", fmt.Sprintf("no label for template %s", *t))
				r.AddError(l, fmt.Sprintf("label %s reserved for the default template", l))
				continue
			}
			r.validateSMSTemplate(l, *t)
		}
	}

	if r.AllowsUserReport() {
		if r.SMSCountry == "" {
			r.AddError("smsCountry", "A default SMS Country must be set when user report is enabled")
		}
	}

	if r.UseSystemEmailConfig && !r.CanUseSystemEmailConfig {
		r.AddError("useSystemEmailConfig", "is not allowed on this realm")
	}

	if r.EmailInviteTemplate != "" {
		if !strings.Contains(r.EmailInviteTemplate, EmailInviteLink) {
			r.AddError("emailInviteLink", fmt.Sprintf("must contain %q", EmailInviteLink))
		}
	}

	if r.EmailPasswordResetTemplate != "" {
		if !strings.Contains(r.EmailPasswordResetTemplate, EmailPasswordResetLink) {
			r.AddError("emailPasswordResetTemplate", fmt.Sprintf("must contain %q", EmailPasswordResetLink))
		}
	}

	if r.EmailVerifyTemplate != "" {
		if !strings.Contains(r.EmailVerifyTemplate, EmailVerifyLink) {
			r.AddError("emailVerifyTemplate", fmt.Sprintf("must contain %q", EmailVerifyLink))
		}
	}

	r.CertificateIssuer = project.TrimSpaceAndNonPrintable(r.CertificateIssuer)
	r.CertificateAudience = project.TrimSpaceAndNonPrintable(r.CertificateAudience)
	if r.UseRealmCertificateKey {
		if r.CertificateIssuer == "" {
			r.AddError("certificateIssuer", "cannot be blank")
		}
		if r.CertificateAudience == "" {
			r.AddError("certificateAudience", "cannot be blank")
		}
	}

	if r.CertificateDuration.AsString != "" {
		if err := r.CertificateDuration.Update(); err != nil {
			r.AddError("certificateDuration", "invalid certificate duration")
		}
	}

	return r.ErrorOrNil()
}

// validateSMSTemplate is a helper method to validate a single SMSTemplate.
// Errors are returned by appending them to the realm's Errorable fields.
func (r *Realm) validateSMSTemplate(label, t string) {
	if !r.EnableENExpress {
		// Check that we have exactly one of [code] or [longcode] as template substitutions.
		if c, lc := strings.Contains(t, SMSCode), strings.Contains(t, SMSLongCode); !(c || lc) || (c && lc) {
			r.AddError("smsTextTemplate", fmt.Sprintf("must contain exactly one of %q or %q", SMSCode, SMSLongCode))
			r.AddError(label, fmt.Sprintf("must contain exactly one of %q or %q", SMSCode, SMSLongCode))
		}
		if strings.Contains(t, SMSENExpressLink) {
			r.AddError("smsTextTemplate", fmt.Sprintf("cannot contain %q because Exposure Notifications Express is not enabled", SMSENExpressLink))
			r.AddError(label, fmt.Sprintf("cannot contain %q", SMSENExpressLink))
		}
	} else {
		if !strings.Contains(t, SMSENExpressLink) {
			r.AddError("smsTextTemplate", fmt.Sprintf("must contain %q", SMSENExpressLink))
			r.AddError(label, fmt.Sprintf("must contain %q", SMSENExpressLink))
		}
		if strings.Contains(t, SMSRegion) {
			r.AddError("smsTextTemplate", fmt.Sprintf("cannot contain %q - this is automatically included in %q", SMSRegion, SMSENExpressLink))
			r.AddError(label, fmt.Sprintf("must contain %q", SMSENExpressLink))
		}
		if strings.Contains(t, SMSLongCode) {
			r.AddError("smsTextTemplate", fmt.Sprintf("cannot contain %q - the long code is automatically included in %q", SMSLongCode, SMSENExpressLink))
			r.AddError(label, fmt.Sprintf("must contain %q", SMSENExpressLink))
		}
	}

	// Check template length.
	if l := len(t); l > SMSTemplateMaxLength {
		r.AddError("smsTextTemplate", fmt.Sprintf("must be %d characters or less, current message is %v characters long", SMSTemplateMaxLength, l))
		r.AddError(label, fmt.Sprintf("must contain %q", SMSENExpressLink))
	}

	// Check expansion length based on settings.
	fakeCode := fmt.Sprintf(fmt.Sprintf("\\%0%d\\%d", r.CodeLength), 0)
	fakeLongCode := fmt.Sprintf(fmt.Sprintf("\\%0%d\\%d", r.LongCodeLength), 0)
	enxDomain := r.enxRedirectDomain()
	expandedSMSText, err := r.BuildSMSText(fakeCode, fakeLongCode, enxDomain, label)
	if err != nil {
		r.AddError("smsTextTemplate", fmt.Sprintf("SMS template expansion failed: %s", err))
		r.AddError(label, fmt.Sprintf("SMS template expansion failed: %s", err))
	}
	if l := len(expandedSMSText); l > SMSTemplateExpansionMax {
		r.AddError("smsTextTemplate", fmt.Sprintf("when expanded, the result message is too long (%v characters). The max expanded message is %v characters", l, SMSTemplateExpansionMax))
		r.AddError(label, fmt.Sprintf("when expanded, the result message is too long (%v characters). The max expanded message is %v characters", l, SMSTemplateExpansionMax))
	}
}

// enxRedirectDomain returns the configured ENX redirect domain for this realm.
func (r *Realm) enxRedirectDomain() string {
	if v := r.enxRedirectDomainOverride; v != "" {
		return v
	}
	return ENXRedirectDomain
}

// GetCodeDurationMinutes is a helper for the HTML rendering to get a round
// minutes value.
func (r *Realm) GetCodeDurationMinutes() int {
	return int(r.CodeDuration.Duration.Minutes())
}

// GetLongCodeDurationHours is a helper for the HTML rendering to get a round
// hours value.
func (r *Realm) GetLongCodeDurationHours() int {
	return int(r.LongCodeDuration.Duration.Hours())
}

// EffectiveMFAMode returns the realm's default MFAMode but first checks if the
// time is in the grace-period (if so, required becomes prompt).
func (r *Realm) EffectiveMFAMode(t time.Time) AuthRequirement {
	if r == nil {
		return MFARequired
	}

	if time.Since(t) <= r.MFARequiredGracePeriod.Duration {
		return MFAOptionalPrompt
	}
	return r.MFAMode
}

// CodesClaimedRatioAnomalous returns true if the ratio of codes issued to codes
// claimed is less than the predicted mean by more than one standard deviation.
func (r *Realm) CodesClaimedRatioAnomalous() bool {
	return r.LastCodesClaimedRatio < r.CodesClaimedRatioMean &&
		r.CodesClaimedRatioMean-r.LastCodesClaimedRatio > r.CodesClaimedRatioStddev
}

// FindVerificationCodeByUUID find a verification codes by UUID. It returns
// NotFound if the UUID is invalid.
func (r *Realm) FindVerificationCodeByUUID(db *Database, uuidStr string) (*VerificationCode, error) {
	// Postgres returns an error if the provided input is not a valid UUID.
	parsed, err := uuid.Parse(uuidStr)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}

	var vc VerificationCode
	if err := db.db.
		Where("uuid = ? AND realm_id = ?", parsed.String(), r.ID).
		First(&vc).Error; err != nil {
		return nil, err
	}
	return &vc, nil
}

// BuildSMSText replaces certain strings with the right values.
func (r *Realm) BuildSMSText(code, longCode string, enxDomain, templateLabel string) (string, error) {
	text := r.SMSTextTemplate
	if templateLabel != "" && templateLabel != DefaultTemplateLabel && r.SMSTextAlternateTemplates != nil {
		if t, has := r.SMSTextAlternateTemplates[templateLabel]; has && t != nil && *t != "" {
			text = *t
		} else {
			return "", fmt.Errorf("no template found for label %s", templateLabel)
		}
	}

	if enxDomain == "" {
		// preserves legacy behavior.
		text = strings.ReplaceAll(text, SMSENExpressLink, fmt.Sprintf("ens://v?r=%s&c=%s", SMSRegion, SMSLongCode))
	} else {
		text = strings.ReplaceAll(text, SMSENExpressLink,
			fmt.Sprintf("https://%s.%s/v?c=%s",
				strings.ToLower(r.RegionCode),
				enxDomain,
				SMSLongCode))
	}
	text = strings.ReplaceAll(text, SMSRegion, r.RegionCode)
	text = strings.ReplaceAll(text, SMSCode, code)
	text = strings.ReplaceAll(text, SMSExpires, fmt.Sprintf("%d", r.GetCodeDurationMinutes()))
	text = strings.ReplaceAll(text, SMSLongCode, longCode)
	text = strings.ReplaceAll(text, SMSLongExpires, fmt.Sprintf("%d", r.GetLongCodeDurationHours()))

	return text, nil
}

// BuildInviteEmail replaces certain strings with the right values for invitations.
func (r *Realm) BuildInviteEmail(inviteLink string) string {
	text := r.EmailInviteTemplate
	text = strings.ReplaceAll(text, EmailInviteLink, inviteLink)
	text = strings.ReplaceAll(text, RealmName, r.Name)
	return text
}

// BuildPasswordResetEmail replaces certain strings with the right values for password reset.
func (r *Realm) BuildPasswordResetEmail(passwordResetLink string) string {
	text := r.EmailPasswordResetTemplate
	text = strings.ReplaceAll(text, EmailPasswordResetLink, passwordResetLink)
	text = strings.ReplaceAll(text, RealmName, r.Name)
	return text
}

// BuildVerifyEmail replaces certain strings with the right values for email verification.
func (r *Realm) BuildVerifyEmail(verifyLink string) string {
	text := r.EmailVerifyTemplate
	text = strings.ReplaceAll(text, EmailVerifyLink, verifyLink)
	text = strings.ReplaceAll(text, RealmName, r.Name)
	return text
}

// SMSConfig returns the SMS configuration for this realm, if one exists. If the
// realm is configured to use the system SMS configuration, that configuration
// is preferred.
func (r *Realm) SMSConfig(db *Database) (*SMSConfig, error) {
	q := db.db.
		Model(&SMSConfig{}).
		Order("is_system DESC").
		Where("realm_id = ?", r.ID)

	if r.UseSystemSMSConfig {
		q = q.Or("is_system IS TRUE")
	}

	var smsConfig SMSConfig
	if err := q.First(&smsConfig).Error; err != nil {
		return nil, err
	}

	// For system configurations, look up the appropriate from number.
	if smsConfig.IsSystem {
		smsFromNumber, err := db.FindSMSFromNumber(r.SMSFromNumberID)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup sms from number: %w", err)
		}
		smsConfig.TwilioFromNumber = smsFromNumber.Value
	}

	return &smsConfig, nil
}

// HasSMSConfig returns true if the realm has an SMS config, false otherwise.
// This does not perform the KMS encryption/decryption, so it's more efficient
// that loading the full SMS config.
func (r *Realm) HasSMSConfig(db *Database) (bool, error) {
	q := db.db.
		Model(&SMSConfig{}).
		Select("id").
		Order("is_system DESC").
		Where("realm_id = ?", r.ID)

	if r.UseSystemSMSConfig {
		q = q.Or("is_system IS TRUE")
	}

	var id []uint64
	if err := q.Pluck("id", &id).Error; err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return len(id) > 0, nil
}

// SMSProvider returns the SMS provider for the realm. If no sms configuration
// exists, it returns nil. If any errors occur creating the provider, they are
// returned.
func (r *Realm) SMSProvider(db *Database) (sms.Provider, error) {
	smsConfig, err := r.SMSConfig(db)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	ctx := context.Background()
	provider, err := sms.ProviderFor(ctx, &sms.Config{
		ProviderType:     smsConfig.ProviderType,
		TwilioAccountSid: smsConfig.TwilioAccountSid,
		TwilioAuthToken:  smsConfig.TwilioAuthToken,
		TwilioFromNumber: smsConfig.TwilioFromNumber,
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// EmailConfig returns the email configuration for this realm, if one exists. If the
// realm is configured to use the system email configuration, that configuration
// is preferred.
func (r *Realm) EmailConfig(db *Database) (*EmailConfig, error) {
	q := db.db.
		Model(&EmailConfig{}).
		Order("is_system DESC").
		Where("realm_id = ?", r.ID)

	if r.UseSystemEmailConfig {
		q = q.Or("is_system IS TRUE")
	}

	var emailConfig EmailConfig
	if err := q.First(&emailConfig).Error; err != nil {
		return nil, err
	}
	return &emailConfig, nil
}

// EmailProvider returns the email provider for the realm. If no email configuration
// exists, it returns nil. If any errors occur creating the provider, they are
// returned.
func (r *Realm) EmailProvider(db *Database) (email.Provider, error) {
	emailConfig, err := r.EmailConfig(db)
	if err != nil {
		return nil, err
	}

	return emailConfig.Provider()
}

// ListAudits returns the list audit events which match the given criteria.
func (r *Realm) ListAudits(db *Database, p *pagination.PageParams, scopes ...Scope) ([]*AuditEntry, *pagination.Paginator, error) {
	scopes = append(scopes, WithAuditRealmID(r.ID))
	return db.ListAudits(p, scopes...)
}

// AbusePreventionEffectiveLimit returns the effective limit, multiplying the limit by the
// limit factor and rounding up.
func (r *Realm) AbusePreventionEffectiveLimit() uint {
	// Only maintain 3 digits of precision, since that's all we do in the
	// database.
	factor := math.Floor(float64(r.AbusePreventionLimitFactor)*100) / 100
	return uint(math.Ceil(float64(r.AbusePreventionLimit) * factor))
}

// CurrentSigningKey returns the currently active certificate signing key, the one marked
// active in the database. If there is more than one active, the most recently
// created one wins. Should not occur due to transactional update.
func (r *Realm) CurrentSigningKey(db *Database) (*SigningKey, error) {
	var signingKey SigningKey
	if err := db.db.
		Where("realm_id = ?", r.ID).
		Where("active = ?", true).
		First(&signingKey).
		Error; err != nil {
		return nil, fmt.Errorf("failed to find signing key: %w", err)
	}
	return &signingKey, nil
}

// CurrentSMSSigningKey returns the currently active SMS signing key, the one
// marked active in the database. There cannot be more than one active key due
// to a database-level constraint.
func (r *Realm) CurrentSMSSigningKey(db *Database) (*SMSSigningKey, error) {
	var signingKey SMSSigningKey
	if err := db.db.
		Where("realm_id = ?", r.ID).
		Where("active = ?", true).
		First(&signingKey).
		Error; err != nil {
		return nil, fmt.Errorf("failed to find signing key: %w", err)
	}
	return &signingKey, nil
}

// SetActiveSigningKey sets a specific signing key to active=true for the realm,
// and transactionally sets all other signing keys to inactive. It accepts the
// database primary key ID but returns the KID of the now-active key.
func (r *Realm) SetActiveSigningKey(db *Database, id uint, actor Auditable) (string, error) {
	return r.setActiveManagedSigningKey(db, id, &SigningKey{}, actor)
}

// SetActiveSMSSigningKey sets a specific signing key to active=true for the realm,
// and transactionally sets all other signing keys to inactive. It accepts the
// database primary key ID but returns the KID of the now-active key.
func (r *Realm) SetActiveSMSSigningKey(db *Database, id uint, actor Auditable) (string, error) {
	return r.setActiveManagedSigningKey(db, id, &SMSSigningKey{}, actor)
}

func (r *Realm) setActiveManagedSigningKey(db *Database, id uint, signingKey RealmManagedKey, actor Auditable) (string, error) {
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Find the key that should be active - do this first to ensure that the
		// provided PK id is actually valid.
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Table(signingKey.Table()).
			Where("id = ?", id).
			Where("realm_id = ?", r.ID).
			First(signingKey).
			Error; err != nil {
			if IsNotFound(err) {
				return fmt.Errorf("%s key to activate does not exist: %w", signingKey.Purpose(), err)
			}
			return fmt.Errorf("failed to find newly active key: %w", err)
		}

		// Mark all other keys as inactive.
		if err := tx.
			Table(signingKey.Table()).
			Where("realm_id = ?", r.ID).
			Where("id != ?", id).
			Where("deleted_at IS NULL").
			Update(map[string]interface{}{"active": false, "updated_at": time.Now().UTC()}).
			Error; err != nil {
			return fmt.Errorf("failed to mark existing %s keys as inactive: %w", signingKey.Purpose(), err)
		}

		// Mark the active key as active.
		signingKey.SetActive(true)
		if err := tx.Save(signingKey).Error; err != nil {
			return fmt.Errorf("failed to mark new %s key as active: %w", signingKey.Purpose(), err)
		}

		// Generate an audit
		audit := BuildAuditEntry(actor, "updated active signing key", signingKey, r.ID)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	}); err != nil {
		return "", err
	}

	return signingKey.GetKID(), nil
}

// ListSigningKeys returns the non-deleted signing keys for a realm
// ordered by created_at desc.
func (r *Realm) ListSigningKeys(db *Database) ([]*SigningKey, error) {
	var keys []*SigningKey
	if err := db.db.
		Model(&SigningKey{}).
		Where("realm_id = ?", r.ID).
		Order("signing_keys.created_at DESC").
		Find(&keys).
		Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListSMSSigningKeys returns the non-deleted signing keys for a realm
// ordered by created_at desc.
func (r *Realm) ListSMSSigningKeys(db *Database) ([]*SMSSigningKey, error) {
	var keys []*SMSSigningKey
	if err := db.db.
		Model(&SMSSigningKey{}).
		Where("realm_id = ?", r.ID).
		Order("sms_signing_keys.created_at DESC").
		Find(&keys).
		Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *Realm) ListAdminPhones(db *Database, p *pagination.PageParams, scopes ...Scope) ([]*NotificationPhone, *pagination.Paginator, error) {
	var raps []*NotificationPhone
	query := db.db.Model(&NotificationPhone{}).
		Unscoped().
		Scopes(scopes...).
		Where("realm_id = ?", r.ID).
		Order("notification_phones.name ASC")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &raps, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return raps, nil, nil
		}
		return nil, nil, err
	}

	return raps, paginator, nil
}

func (r *Realm) ListAuthorizedApps(db *Database, p *pagination.PageParams, scopes ...Scope) ([]*AuthorizedApp, *pagination.Paginator, error) {
	var authApps []*AuthorizedApp
	query := db.db.Model(&AuthorizedApp{}).
		Unscoped().
		Scopes(scopes...).
		Where("realm_id = ?", r.ID).
		Order("LOWER(authorized_apps.name)")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &authApps, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return authApps, nil, nil
		}
		return nil, nil, err
	}

	return authApps, paginator, nil
}

// FindAdminPhone finds the realm admin phone number by the given id associated to the realm.
func (r *Realm) FindAdminPhone(db *Database, id interface{}) (*NotificationPhone, error) {
	var app NotificationPhone
	if err := db.db.
		Unscoped().
		Model(NotificationPhone{}).
		Order("LOWER(name) ASC").
		Where("id = ? AND realm_id = ?", id, r.ID).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// FindAuthorizedApp finds the authorized app by the given id associated to the
// realm.
func (r *Realm) FindAuthorizedApp(db *Database, id interface{}) (*AuthorizedApp, error) {
	var app AuthorizedApp
	if err := db.db.
		Unscoped().
		Model(AuthorizedApp{}).
		Order("LOWER(name) ASC").
		Where("id = ? AND realm_id = ?", id, r.ID).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListMobileApps gets all the mobile apps for the realm.
func (r *Realm) ListMobileApps(db *Database, p *pagination.PageParams, scopes ...Scope) ([]*MobileApp, *pagination.Paginator, error) {
	var mobileApps []*MobileApp
	query := db.db.
		Unscoped().
		Model(&MobileApp{}).
		Scopes(scopes...).
		Where("realm_id = ?", r.ID).
		Order("mobile_apps.deleted_at DESC, LOWER(mobile_apps.name)")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &mobileApps, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return mobileApps, nil, nil
		}
		return nil, nil, err
	}

	return mobileApps, paginator, nil
}

// FindMobileApp finds the mobile app by the given id associated with the realm.
func (r *Realm) FindMobileApp(db *Database, id interface{}) (*MobileApp, error) {
	var app MobileApp
	if err := db.db.
		Unscoped().
		Model(MobileApp{}).
		Where("id = ?", id).
		Where("realm_id = ?", r.ID).
		First(&app).
		Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListMemberships lists the realm's memberships.
func (r *Realm) ListMemberships(db *Database, p *pagination.PageParams, scopes ...Scope) ([]*Membership, *pagination.Paginator, error) {
	var memberships []*Membership
	query := db.db.
		Preload("Realm").
		Preload("User").
		Model(&Membership{}).
		Scopes(scopes...).
		Where("realm_id = ?", r.ID).
		Where("realms.deleted_at IS NULL").
		Where("users.deleted_at IS NULL").
		Joins("JOIN realms ON realms.id = memberships.realm_id").
		Joins("JOIN users ON users.id = memberships.user_id").
		Order("LOWER(users.name)")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &memberships, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return memberships, nil, nil
		}
		return nil, nil, err
	}

	return memberships, paginator, nil
}

// MembershipPermissionMap returns a map where the key is the ID of a user and
// the value is the permissions for that user.
func (r *Realm) MembershipPermissionMap(db *Database) (map[uint]rbac.Permission, error) {
	var memberships []*Membership
	if err := db.db.
		Model(&Membership{}).
		Find(&memberships).Error; err != nil {
		if !IsNotFound(err) {
			return nil, err
		}
	}

	m := make(map[uint]rbac.Permission, len(memberships))
	for _, v := range memberships {
		m[v.UserID] = v.Permissions
	}
	return m, nil
}

// FindUser finds the given user in the realm by ID.
func (r *Realm) FindUser(db *Database, id interface{}) (*User, error) {
	var user User
	if err := db.db.
		Table("users").
		Joins("INNER JOIN memberships ON user_id = ? AND realm_id = ?", id, r.ID).
		Find(&user, "users.id = ?", id).
		Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ValidTestType returns true if the given test type string is valid for this
// realm, false otherwise.
func (r *Realm) ValidTestType(typ string) bool {
	switch project.TrimSpace(strings.ToLower(typ)) {
	case "confirmed":
		return r.AllowedTestTypes&TestTypeConfirmed != 0
	case "likely":
		return r.AllowedTestTypes&TestTypeLikely != 0
	case "negative":
		return r.AllowedTestTypes&TestTypeNegative != 0
	case "user-report":
		return r.AllowedTestTypes&TestTypeUserReport != 0
	default:
		return false
	}
}

func (db *Database) FindRealmByRegion(region string) (*Realm, error) {
	var realm Realm
	if err := db.db.
		Model(&Realm{}).
		Where("region_code = ?", strings.ToUpper(region)).
		First(&realm).
		Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) FindRealmByName(name string) (*Realm, error) {
	var realm Realm

	if err := db.db.Where("name = ?", name).First(&realm).Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

func (db *Database) FindRealm(id interface{}) (*Realm, error) {
	var realm Realm
	if err := db.db.
		Where("id = ?", id).
		First(&realm).
		Error; err != nil {
		return nil, err
	}
	return &realm, nil
}

// FindRealmByRegionOrID finds the realm by the given ID or region code.
func (db *Database) FindRealmByRegionOrID(val string) (*Realm, error) {
	if project.AllDigits(val) {
		return db.FindRealm(val)
	}
	return db.FindRealmByRegion(val)
}

// ListRealms lists all available realms in the system.
func (db *Database) ListRealms(p *pagination.PageParams, scopes ...Scope) ([]*Realm, *pagination.Paginator, error) {
	var realms []*Realm
	query := db.db.
		Model(&Realm{}).
		Scopes(scopes...).
		Order("name ASC")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &realms, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return realms, nil, nil
		}
		return nil, nil, err
	}

	return realms, paginator, nil
}

func (r *Realm) AuditID() string {
	return fmt.Sprintf("realms:%d", r.ID)
}

func (r *Realm) AuditDisplay() string {
	return r.Name
}

func (db *Database) SaveRealm(r *Realm, actor Auditable) error {
	if r == nil {
		return fmt.Errorf("provided realm is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing Realm
		if err := tx.
			Model(&Realm{}).
			Where("id = ?", r.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing realm: %w", err)
		}

		// Save the realm
		if err := tx.Save(r).Error; err != nil {
			switch {
			case IsUniqueViolation(err, "uix_realms_name"):
				r.AddError("name", "must be unique")
				return ErrValidationFailed
			case IsUniqueViolation(err, "uix_realms_region_code"):
				r.AddError("regionCode", "must be unique")
				return ErrValidationFailed
			}
			return err
		}

		// Brand new realm?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created realm", r, r.ID)
			audits = append(audits, audit)
		} else {
			if existing.Name != r.Name {
				audit := BuildAuditEntry(actor, "updated realm name", r, r.ID)
				audit.Diff = stringDiff(existing.Name, r.Name)
				audits = append(audits, audit)
			}

			if existing.RegionCode != r.RegionCode {
				audit := BuildAuditEntry(actor, "updated region code", r, r.ID)
				audit.Diff = stringDiff(existing.RegionCode, r.RegionCode)
				audits = append(audits, audit)
			}

			if existing.WelcomeMessage != r.WelcomeMessage {
				audit := BuildAuditEntry(actor, "updated welcome message", r, r.ID)
				audit.Diff = stringDiff(existing.WelcomeMessage, r.WelcomeMessage)
				audits = append(audits, audit)
			}

			if existing.CodeLength != r.CodeLength {
				audit := BuildAuditEntry(actor, "updated code length", r, r.ID)
				audit.Diff = uintDiff(existing.CodeLength, r.CodeLength)
				audits = append(audits, audit)
			}

			if existing.CodeDuration != r.CodeDuration {
				audit := BuildAuditEntry(actor, "updated code duration", r, r.ID)
				audit.Diff = stringDiff(existing.CodeDuration.AsString, r.CodeDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.LongCodeLength != r.LongCodeLength {
				audit := BuildAuditEntry(actor, "updated long code length", r, r.ID)
				audit.Diff = uintDiff(existing.LongCodeLength, r.LongCodeLength)
				audits = append(audits, audit)
			}

			if existing.LongCodeDuration != r.LongCodeDuration {
				audit := BuildAuditEntry(actor, "updated long code duration", r, r.ID)
				audit.Diff = stringDiff(existing.LongCodeDuration.AsString, r.LongCodeDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.SMSTextTemplate != r.SMSTextTemplate {
				audit := BuildAuditEntry(actor, "updated SMS template", r, r.ID)
				audit.Diff = stringDiff(existing.SMSTextTemplate, r.SMSTextTemplate)
				audits = append(audits, audit)
			}

			if existing.SMSCountry != r.SMSCountry {
				audit := BuildAuditEntry(actor, "updated SMS country", r, r.ID)
				audit.Diff = stringDiff(existing.SMSCountry, r.SMSCountry)
				audits = append(audits, audit)
			}

			if existing.CanUseSystemSMSConfig != r.CanUseSystemSMSConfig {
				audit := BuildAuditEntry(actor, "updated ability to use system SMS config", r, r.ID)
				audit.Diff = boolDiff(existing.CanUseSystemSMSConfig, r.CanUseSystemSMSConfig)
				audits = append(audits, audit)
			}

			if existing.UseSystemSMSConfig != r.UseSystemSMSConfig {
				audit := BuildAuditEntry(actor, "updated use system SMS config", r, r.ID)
				audit.Diff = boolDiff(existing.UseSystemSMSConfig, r.UseSystemSMSConfig)
				audits = append(audits, audit)
			}

			if existing.UseAuthenticatedSMS != r.UseAuthenticatedSMS {
				audit := BuildAuditEntry(actor, "updated use authenticated SMS", r, r.ID)
				audit.Diff = boolDiff(existing.UseAuthenticatedSMS, r.UseAuthenticatedSMS)
				audits = append(audits, audit)
			}

			if existing.EmailInviteTemplate != r.EmailInviteTemplate {
				audit := BuildAuditEntry(actor, "updated email invite template", r, r.ID)
				audit.Diff = stringDiff(existing.EmailInviteTemplate, r.EmailInviteTemplate)
				audits = append(audits, audit)
			}

			if existing.EmailPasswordResetTemplate != r.EmailPasswordResetTemplate {
				audit := BuildAuditEntry(actor, "updated email password reset template", r, r.ID)
				audit.Diff = stringDiff(existing.EmailPasswordResetTemplate, r.EmailPasswordResetTemplate)
				audits = append(audits, audit)
			}

			if existing.EmailVerifyTemplate != r.EmailVerifyTemplate {
				audit := BuildAuditEntry(actor, "updated email verify template", r, r.ID)
				audit.Diff = stringDiff(existing.EmailVerifyTemplate, r.EmailVerifyTemplate)
				audits = append(audits, audit)
			}

			if existing.CanUseSystemEmailConfig != r.CanUseSystemEmailConfig {
				audit := BuildAuditEntry(actor, "updated ability to use system email config", r, r.ID)
				audit.Diff = boolDiff(existing.CanUseSystemEmailConfig, r.CanUseSystemEmailConfig)
				audits = append(audits, audit)
			}

			if existing.UseSystemEmailConfig != r.UseSystemEmailConfig {
				audit := BuildAuditEntry(actor, "updated use system email config", r, r.ID)
				audit.Diff = boolDiff(existing.UseSystemEmailConfig, r.UseSystemEmailConfig)
				audits = append(audits, audit)
			}

			if existing.MFAMode != r.MFAMode {
				audit := BuildAuditEntry(actor, "updated MFA mode", r, r.ID)
				audit.Diff = stringDiff(existing.MFAMode.String(), r.MFAMode.String())
				audits = append(audits, audit)
			}

			if existing.MFARequiredGracePeriod != r.MFARequiredGracePeriod {
				audit := BuildAuditEntry(actor, "updated MFA required grace period", r, r.ID)
				audit.Diff = stringDiff(existing.MFARequiredGracePeriod.AsString, r.MFARequiredGracePeriod.AsString)
				audits = append(audits, audit)
			}

			if existing.EmailVerifiedMode != r.EmailVerifiedMode {
				audit := BuildAuditEntry(actor, "updated email verification mode", r, r.ID)
				audit.Diff = stringDiff(existing.EmailVerifiedMode.String(), r.EmailVerifiedMode.String())
				audits = append(audits, audit)
			}

			if existing.PasswordRotationPeriodDays != r.PasswordRotationPeriodDays {
				audit := BuildAuditEntry(actor, "updated password rotation period", r, r.ID)
				audit.Diff = uintDiff(existing.PasswordRotationPeriodDays, r.PasswordRotationPeriodDays)
				audits = append(audits, audit)
			}

			if existing.PasswordRotationWarningDays != r.PasswordRotationWarningDays {
				audit := BuildAuditEntry(actor, "updated password rotation warning", r, r.ID)
				audit.Diff = uintDiff(existing.PasswordRotationWarningDays, r.PasswordRotationWarningDays)
				audits = append(audits, audit)
			}

			if then, now := existing.AllowedCIDRsAdminAPI, r.AllowedCIDRsAdminAPI; !reflect.DeepEqual(then, now) {
				audit := BuildAuditEntry(actor, "updated adminapi allowed cidrs", r, r.ID)
				audit.Diff = stringSliceDiff(then, now)
				audits = append(audits, audit)
			}

			if then, now := existing.AllowedCIDRsAPIServer, r.AllowedCIDRsAPIServer; !reflect.DeepEqual(then, now) {
				audit := BuildAuditEntry(actor, "updated apiserver allowed cidrs", r, r.ID)
				audit.Diff = stringSliceDiff(then, now)
				audits = append(audits, audit)
			}

			if then, now := existing.AllowedCIDRsServer, r.AllowedCIDRsServer; !reflect.DeepEqual(then, now) {
				audit := BuildAuditEntry(actor, "updated server allowed cidrs", r, r.ID)
				audit.Diff = stringSliceDiff(then, now)
				audits = append(audits, audit)
			}

			if existing.AllowedTestTypes != r.AllowedTestTypes {
				audit := BuildAuditEntry(actor, "updated allowed test types", r, r.ID)
				audit.Diff = stringDiff(existing.AllowedTestTypes.Display(), r.AllowedTestTypes.Display())
				audits = append(audits, audit)
			}

			if existing.RequireDate != r.RequireDate {
				audit := BuildAuditEntry(actor, "updated require date", r, r.ID)
				audit.Diff = boolDiff(existing.RequireDate, r.RequireDate)
				audits = append(audits, audit)
			}

			if existing.UseRealmCertificateKey != r.UseRealmCertificateKey {
				audit := BuildAuditEntry(actor, "updated use realm certificate key", r, r.ID)
				audit.Diff = boolDiff(existing.UseRealmCertificateKey, r.UseRealmCertificateKey)
				audits = append(audits, audit)
			}

			if existing.CertificateIssuer != r.CertificateIssuer {
				audit := BuildAuditEntry(actor, "updated certificate issuer", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateIssuer, r.CertificateIssuer)
				audits = append(audits, audit)
			}

			if existing.CertificateAudience != r.CertificateAudience {
				audit := BuildAuditEntry(actor, "updated certificate audience", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateAudience, r.CertificateAudience)
				audits = append(audits, audit)
			}

			if existing.CertificateDuration != r.CertificateDuration {
				audit := BuildAuditEntry(actor, "updated certificate duration", r, r.ID)
				audit.Diff = stringDiff(existing.CertificateDuration.AsString, r.CertificateDuration.AsString)
				audits = append(audits, audit)
			}

			if existing.AutoRotateCertificateKey != r.AutoRotateCertificateKey {
				audit := BuildAuditEntry(actor, "updated auto-rotate certificate keys", r, r.ID)
				audit.Diff = boolDiff(existing.AutoRotateCertificateKey, r.AutoRotateCertificateKey)
				audits = append(audits, audit)
			}

			if existing.EnableENExpress != r.EnableENExpress {
				audit := BuildAuditEntry(actor, "updated enable ENX", r, r.ID)
				audit.Diff = boolDiff(existing.EnableENExpress, r.EnableENExpress)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionEnabled != r.AbusePreventionEnabled {
				audit := BuildAuditEntry(actor, "updated enable abuse prevention", r, r.ID)
				audit.Diff = boolDiff(existing.AbusePreventionEnabled, r.AbusePreventionEnabled)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionLimit != r.AbusePreventionLimit {
				audit := BuildAuditEntry(actor, "updated abuse prevention limit", r, r.ID)
				audit.Diff = uintDiff(existing.AbusePreventionLimit, r.AbusePreventionLimit)
				audits = append(audits, audit)
			}

			if existing.AbusePreventionLimitFactor != r.AbusePreventionLimitFactor {
				audit := BuildAuditEntry(actor, "updated abuse prevention limit factor", r, r.ID)
				audit.Diff = float32Diff(existing.AbusePreventionLimitFactor, r.AbusePreventionLimitFactor)
				audits = append(audits, audit)
			}
		}

		// Save all audits
		for _, audit := range audits {
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audits: %w", err)
			}
		}

		return nil
	})
}

func (r *Realm) CreateRealmAdminPhone(db *Database, rap *NotificationPhone, actor Auditable) error {
	return db.SaveRealmAdminPhone(r, rap, actor)
}

// CreateAuthorizedApp generates a new API key and assigns it to the specified
// app. Note that the API key is NOT stored in the database, only a hash. The
// only time the API key is available is as the string return parameter from
// invoking this function.
func (r *Realm) CreateAuthorizedApp(db *Database, app *AuthorizedApp, actor Auditable) (string, error) {
	fullAPIKey, err := db.GenerateAPIKey(r.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	parts := strings.SplitN(fullAPIKey, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("internal error, key is invalid")
	}
	apiKey := parts[0]

	hmacedKey, err := db.GenerateAPIKeyHMAC(apiKey)
	if err != nil {
		return "", fmt.Errorf("failed to create hmac: %w", err)
	}

	app.RealmID = r.ID
	app.APIKey = hmacedKey
	app.APIKeyPreview = apiKey[:6]

	if err := db.SaveAuthorizedApp(app, actor); err != nil {
		return "", err
	}
	return fullAPIKey, nil
}

func (r *Realm) CanUpgradeToRealmSigningKeys() bool {
	return r.CertificateIssuer != "" && r.CertificateAudience != ""
}

// certificateSigningKMSKeyName is the unique name of the certificate signing
// key in the upstream KMS.
func (r *Realm) certificateSigningKMSKeyName() string {
	return fmt.Sprintf("realm-%d", r.ID)
}

// smsSigningKMSKeyName is the unique name of the SMS signing key in the
// upstream KMS.
func (r *Realm) smsSigningKMSKeyName() string {
	return fmt.Sprintf("realm-sms-%d", r.ID)
}

// CreateSigningKeyVersion creates a new signing key version on the key manager
// and saves a reference to the new key version in the database. If creating the
// key in the key manager fails, the database is not updated. However, if
// updating the signing key in the database fails, the key is NOT deleted from
// the key manager.
func (r *Realm) CreateSigningKeyVersion(ctx context.Context, db *Database, actor Auditable) (string, error) {
	return r.createManagedSigningKey(ctx, db, r.certificateSigningKMSKeyName(), &SigningKey{}, actor)
}

// CreateSMSSigningKeyVersion creates a new SMS signing key version on the key manager
// and saves a reference to the new key version in the database.
func (r *Realm) CreateSMSSigningKeyVersion(ctx context.Context, db *Database, actor Auditable) (string, error) {
	return r.createManagedSigningKey(ctx, db, r.smsSigningKMSKeyName(), &SMSSigningKey{}, actor)
}

func (r *Realm) createManagedSigningKey(ctx context.Context, db *Database, keyID string, signingKey RealmManagedKey, actor Auditable) (string, error) {
	manager := db.signingKeyManager
	if manager == nil {
		return "", ErrNoSigningKeyManager
	}

	parent := db.config.KeyRing
	if parent == "" {
		return "", fmt.Errorf("missing DB_KEYRING")
	}

	name := keyID
	if name == "" {
		return "", fmt.Errorf("missing key name")
	}

	// Check how many non-deleted signing keys currently exist. There's a limit on
	// the number of "active" signing keys to help protect realms against
	// excessive costs.
	var count int64
	if err := db.db.
		Table(signingKey.Table()).
		Where("realm_id = ?", r.ID).
		Where("deleted_at IS NULL").
		Count(&count).
		Error; err != nil {
		if !IsNotFound(err) {
			return "", fmt.Errorf("failed to count existing %s signing keys: %w", signingKey.Purpose(), err)
		}
	}
	if max := db.config.MaxKeyVersions; count >= max {
		return "", fmt.Errorf("too many available %s signing keys (maximum: %d)", signingKey.Purpose(), max)
	}

	// Create the parent key - this interface does not return an error if the key
	// already exists, so this is safe to run each time.
	keyName, err := manager.CreateSigningKey(ctx, parent, name)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Create a new key version. This returns the full version name.
	version, err := manager.CreateKeyVersion(ctx, keyName)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key version: %w", err)
	}

	// Drop a log message for debugging.
	db.logger.Debugw("provisioned new signing key for realm",
		"realm_id", r.ID,
		"purpose", signingKey.Purpose(),
		"key_id", version)

	// Save the reference to the key in the database. This is done in a
	// transaction to avoid a race where keys are being created simultaneously and
	// both are set to active.
	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Look and see if there are existing signing keys for this realm. We do
		// this to determine if the new key should be set to "active" automatically
		// or if the user needs to take manual action to move the pointer.
		var count int64
		if err := tx.
			Table(signingKey.Table()).
			Where("realm_id = ?", r.ID).
			Count(&count).
			Error; err != nil {
			if !IsNotFound(err) {
				return fmt.Errorf("failed to check for existing %s signing keys: %w", signingKey.Purpose(), err)
			}
		}

		// Create the new key.
		signingKey.SetRealmID(r.ID)
		signingKey.SetManagedKeyID(version)
		signingKey.SetActive(count == 0)

		// Save the key.
		if err := tx.Save(signingKey).Error; err != nil {
			return fmt.Errorf("failed to save reference to %s signing key: %w", signingKey.Purpose(), err)
		}

		// Generate an audit
		audit := BuildAuditEntry(actor, "created signing key", signingKey, r.ID)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	}); err != nil {
		return "", err
	}

	return signingKey.GetKID(), nil
}

// DestroySigningKeyVersion destroys the given key version in both the database
// and the key manager. ID is the primary key ID from the database. If the id
// does not exist, it does nothing.
func (r *Realm) DestroySigningKeyVersion(ctx context.Context, db *Database, id interface{}, actor Auditable) error {
	return r.destroyManagedSigningKey(ctx, db, id, &SigningKey{}, actor)
}

func (r *Realm) DestroySMSSigningKeyVersion(ctx context.Context, db *Database, id interface{}, actor Auditable) error {
	return r.destroyManagedSigningKey(ctx, db, id, &SMSSigningKey{}, actor)
}

func (r *Realm) destroyManagedSigningKey(ctx context.Context, db *Database, id interface{}, signingKey ManagedKey, actor Auditable) error {
	manager := db.signingKeyManager
	if manager == nil {
		return ErrNoSigningKeyManager
	}

	if err := db.db.Transaction(func(tx *gorm.DB) error {
		// Load the signing key to ensure it actually exists.
		if err := tx.
			Set("gorm:query_option", "FOR UPDATE").
			Table(signingKey.Table()).
			Where("id = ?", id).
			Where("realm_id = ?", r.ID).
			First(signingKey).
			Error; err != nil {
			if IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to load %s signing key: %w", signingKey.Purpose(), err)
		}

		if signingKey.IsActive() {
			return fmt.Errorf("cannot destroy active %s signing key", signingKey.Purpose())
		}

		// Delete the signing key from the key manager - we want to do this in the
		// transaction so, if it fails, we can rollback and try again.
		if err := manager.DestroyKeyVersion(ctx, signingKey.ManagedKeyID()); err != nil {
			return fmt.Errorf("failed to destroy %s signing key in key manager: %w", signingKey.Purpose(), err)
		}

		// Successfully deleted from the key manager, now remove the record.
		if err := tx.Delete(signingKey).Error; err != nil {
			return fmt.Errorf("successfully destroyed %s signing key in key manager, "+
				"but failed to delete signing key from database: %w", signingKey.Purpose(), err)
		}

		// Generate an audit
		audit := BuildAuditEntry(actor, "destroyed signing key", signingKey, r.ID)
		if err := tx.Save(audit).Error; err != nil {
			return fmt.Errorf("failed to save audits: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to destroy %s signing key version: %w", signingKey.Purpose(), err)
	}

	return nil
}

// Stats returns the usage statistics for this realm. If no stats exist, returns
// an empty array.
func (r *Realm) Stats(db *Database) (RealmStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	sql := `
		SELECT
			d.date AS date,
			$1 AS realm_id,
			COALESCE(s.codes_issued, 0) AS codes_issued,
			COALESCE(s.codes_claimed, 0) AS codes_claimed,
			COALESCE(s.codes_invalid, 0) AS codes_invalid,
			COALESCE(s.user_reports_issued, 0) AS user_reports_issued,
			COALESCE(s.user_reports_claimed, 0) AS user_reports_claimed,
			COALESCE(s.tokens_claimed, 0) AS tokens_claimed,
			COALESCE(s.tokens_invalid, 0) AS tokens_invalid,
			COALESCE(s.user_report_tokens_claimed, 0) AS user_report_tokens_claimed,
			COALESCE(s.code_claim_age_distribution, array[]::integer[]) AS code_claim_age_distribution,
			COALESCE(s.code_claim_mean_age, 0) AS code_claim_mean_age,
			COALESCE(s.codes_invalid_by_os, array[0,0,0]::bigint[]) AS codes_invalid_by_os
		FROM (
			SELECT date::date FROM generate_series($2, $3, '1 day'::interval) date
		) d
		LEFT JOIN realm_stats s ON s.realm_id = $1 AND s.date = d.date
		ORDER BY date DESC`

	var stats []*RealmStat
	if err := db.db.Raw(sql, r.ID, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}

	return stats, nil
}

// StatsCached is stats, but cached.
func (r *Realm) StatsCached(ctx context.Context, db *Database, cacher cache.Cacher) (RealmStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats RealmStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm",
		Key:       strconv.FormatUint(uint64(r.ID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return r.Stats(db)
	}); err != nil {
		return nil, err
	}

	return stats, nil
}

// ExternalIssuerStats returns the external issuer stats for this realm. If no
// stats exist, returns an empty slice.
func (r *Realm) ExternalIssuerStats(db *Database) (ExternalIssuerStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	// Pull the stats by generating the full date range and full list of external
	// issuers that generated data in that range, then join on stats. This will
	// ensure we have a full list (with values of 0 where appropriate) to ensure
	// continuity in graphs.
	sql := `
		SELECT
			d.date AS date,
			$1 AS realm_id,
			d.issuer_id AS issuer_id,
			COALESCE(s.codes_issued, 0) AS codes_issued
		FROM (
			SELECT
				d.date AS date,
				i.issuer_id AS issuer_id
			FROM generate_series($2, $3, '1 day'::interval) d
			CROSS JOIN (
				SELECT DISTINCT(issuer_id)
				FROM external_issuer_stats
				WHERE realm_id = $1 AND date >= $2 AND date <= $3
			) AS i
		) d
		LEFT JOIN external_issuer_stats s ON s.realm_id = $1 AND s.issuer_id = d.issuer_id AND s.date = d.date
		ORDER BY date DESC, issuer_id`

	var stats []*ExternalIssuerStat
	if err := db.db.Raw(sql, r.ID, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// ExternalIssuerStatsCached is stats, but cached.
func (r *Realm) ExternalIssuerStatsCached(ctx context.Context, db *Database, cacher cache.Cacher) (ExternalIssuerStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats ExternalIssuerStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:per_external_issuer",
		Key:       strconv.FormatUint(uint64(r.ID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return r.ExternalIssuerStats(db)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// SMSErrorStats returns the sms error stats for this realm.
func (r *Realm) SMSErrorStats(db *Database) (SMSErrorStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	// Ensure we have a full list (with values of 0 where appropriate) to ensure
	// continuity in graphs.
	sql := `
		SELECT
			d.date AS date,
			$1 AS realm_id,
			d.error_code AS error_code,
			COALESCE(s.quantity, 0) AS quantity
		FROM (
			SELECT
				d.date AS date,
				i.error_code AS error_code
			FROM generate_series($2, $3, '1 day'::interval) d
			CROSS JOIN (
				SELECT DISTINCT(error_code)
				FROM sms_error_stats
				WHERE realm_id = $1 AND date >= $2 AND date <= $3
			) AS i
		) d
		LEFT JOIN sms_error_stats s ON s.realm_id = $1 AND s.error_code = d.error_code AND s.date = d.date
		ORDER BY date DESC, error_code`

	var stats []*SMSErrorStat
	if err := db.db.Raw(sql, r.ID, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// SMSErrorStatsCached is stats, but cached.
func (r *Realm) SMSErrorStatsCached(ctx context.Context, db *Database, cacher cache.Cacher) (SMSErrorStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats SMSErrorStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:sms_error_stats",
		Key:       strconv.FormatUint(uint64(r.ID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return r.SMSErrorStats(db)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// UserStats returns the stats by user.
func (r *Realm) UserStats(db *Database) (RealmUserStats, error) {
	stop := timeutils.UTCMidnight(time.Now())
	start := stop.Add(project.StatsDisplayDays * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	// Pull the stats by generating the full date range and full list of users
	// that generated data in that range, then join on stats. This will ensure we
	// have a full list (with values of 0 where appropriate) to ensure continuity
	// in graphs.
	sql := `
		SELECT
			d.date AS date,
			$1 AS realm_id,
			d.user_id AS user_id,
			u.name AS name,
			u.email AS email,
			COALESCE(s.codes_issued, 0) AS codes_issued
		FROM (
			SELECT
				d.date AS date,
				i.user_id AS user_id
			FROM generate_series($2, $3, '1 day'::interval) d
			CROSS JOIN (
				SELECT DISTINCT(user_id)
				FROM user_stats
				WHERE realm_id = $1 AND date >= $2 AND date <= $3
			) AS i
		) d
		LEFT JOIN user_stats s ON s.realm_id = $1 AND s.user_id = d.user_id AND s.date = d.date
		LEFT JOIN users u ON u.id = d.user_id
		ORDER BY date DESC, u.name`

	var stats []*RealmUserStat
	if err := db.db.Raw(sql, r.ID, start, stop).Scan(&stats).Error; err != nil {
		if IsNotFound(err) {
			return stats, nil
		}
		return nil, err
	}
	return stats, nil
}

// UserStatsCached is stats, but cached.
func (r *Realm) UserStatsCached(ctx context.Context, db *Database, cacher cache.Cacher) (RealmUserStats, error) {
	if cacher == nil {
		return nil, fmt.Errorf("cacher cannot be nil")
	}

	var stats RealmUserStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:per_user",
		Key:       strconv.FormatUint(uint64(r.ID), 10),
	}
	if err := cacher.Fetch(ctx, cacheKey, &stats, 30*time.Minute, func() (interface{}, error) {
		return r.UserStats(db)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

// RenderWelcomeMessage message renders the realm's welcome message.
func (r *Realm) RenderWelcomeMessage() string {
	msg := project.TrimSpace(r.WelcomeMessage)
	if msg == "" {
		return ""
	}

	raw := blackfriday.Run([]byte(msg))
	return string(bluemonday.UGCPolicy().SanitizeBytes(raw))
}

// QuotaKey returns the unique and consistent key to use for storing quota data
// for this realm, given the provided HMAC key.
func (r *Realm) QuotaKey(hmacKey []byte) (string, error) {
	dig, err := digest.HMACUint(r.ID, hmacKey)
	if err != nil {
		return "", fmt.Errorf("failed to create realm quota key: %w", err)
	}
	return fmt.Sprintf("realm:quota:%s", dig), nil
}

// RecordChaffEvent records that the realm received a chaff event on the given
// date. This is not a counter, but a boolean: chaff was either received or it
// wasn't. This is used to help server operators identify if an app is not
// sending chaff requests.
func (r *Realm) RecordChaffEvent(db *Database, t time.Time) error {
	t = timeutils.UTCMidnight(t)

	realmSQL := `
		INSERT INTO realm_chaff_events(realm_id, date)
			VALUES ($1, $2)
		ON CONFLICT (realm_id, date) DO NOTHING`
	if err := db.db.Exec(realmSQL, r.ID, t).Error; err != nil {
		return fmt.Errorf("failed to record chaff event: %w", err)
	}

	return nil
}

// ListChaffEvents returns the chaff events for the realm, ordered by date.
func (r *Realm) ListChaffEvents(db *Database) ([]*RealmChaffEvent, error) {
	stop := timeutils.UTCMidnight(time.Now().UTC())
	start := stop.Add(6 * -24 * time.Hour)
	if start.After(stop) {
		return nil, ErrBadDateRange
	}

	sql := `
		SELECT
			d.date AS date,
			CASE
				WHEN s.realm_id IS NULL THEN false
				ELSE true
			END AS present
		FROM (
			SELECT date::date FROM generate_series($2, $3, '1 day'::interval) date
		) d
		LEFT JOIN realm_chaff_events s ON s.realm_id = $1 AND s.date = d.date
		ORDER BY date DESC`

	var events []*RealmChaffEvent
	if err := db.db.Raw(sql, r.ID, start, stop).Scan(&events).Error; err != nil {
		if IsNotFound(err) {
			return events, nil
		}
		return nil, err
	}
	return events, nil
}

// ToCIDRList converts the newline-separated and/or comma-separated CIDR list
// into an array of strings.
func ToCIDRList(s string) ([]string, error) {
	var cidrs []string
	for _, line := range strings.Split(s, "\n") {
		for _, v := range strings.Split(line, ",") {
			v = project.TrimSpace(v)

			// Ignore blanks
			if v == "" {
				continue
			}

			// If there's no /, assume the most specific. This is intentionally
			// rudimentary.
			if !strings.Contains(v, "/") {
				if strings.Contains(v, ":") {
					v = fmt.Sprintf("%s/128", v)
				} else {
					v = fmt.Sprintf("%s/32", v)
				}
			}

			// Basic sanity checking.
			if _, _, err := net.ParseCIDR(v); err != nil {
				return nil, err
			}

			cidrs = append(cidrs, v)
		}
	}

	sort.Strings(cidrs)
	return cidrs, nil
}
