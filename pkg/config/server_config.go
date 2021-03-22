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

package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"

	"github.com/google/exposure-notifications-server/pkg/observability"

	firebase "firebase.google.com/go"
	"github.com/sethvargo/go-envconfig"
)

var _ IssueAPIConfig = (*ServerConfig)(nil)

// PasswordRequirementsConfig represents the password complexity requirements for the server.
type PasswordRequirementsConfig struct {
	Length    int `env:"MIN_PWD_LENGTH,default=8"`
	Uppercase int `env:"MIN_PWD_UPPER,default=1"`
	Lowercase int `env:"MIN_PWD_LOWER,default=1"`
	Number    int `env:"MIN_PWD_DIGITS,default=1"`
	Special   int `env:"MIN_PWD_SPECIAL,default=1"`
}

// HasRequirements is true if any requirements are set.
func (c *PasswordRequirementsConfig) HasRequirements() bool {
	return c.Length > 0 || c.Uppercase > 0 || c.Lowercase > 0 || c.Number > 0 || c.Special > 0
}

// ServerConfig represents the environment based config for the server.
type ServerConfig struct {
	Firebase      FirebaseConfig
	Database      database.Config
	Observability observability.Config
	Cache         cache.Config
	Features      FeatureConfig

	// SystemNotice is an optional notice that will be presented at the top of all
	// pages on the UI if provided. It supports markdown syntax.
	SystemNotice string `env:"SYSTEM_NOTICE"`
	systemNotice string

	// Certificate signing key settings, needed for public key / settings display.
	CertificateSigning CertificateSigningConfig

	// SMSSigning defines the SMS signing configuration.
	SMSSigning SMSSigningConfig

	Port string `env:"PORT,default=8080"`

	// Login Config
	SessionDuration    time.Duration `env:"SESSION_DURATION, default=20h"`
	SessionIdleTimeout time.Duration `env:"SESSION_IDLE_TIMEOUT, default=20m"`
	RevokeCheckPeriod  time.Duration `env:"REVOKE_CHECK_DURATION, default=5m"`

	// Password Config
	PasswordRequirements PasswordRequirementsConfig

	// CookieKeys is a slice of bytes. The first is 64 bytes, the second is 32.
	// They should be base64-encoded.
	CookieKeys Base64ByteSlice `env:"COOKIE_KEYS,required"`

	// CookieDomain is the domain for which cookie should be valid.
	CookieDomain string `env:"COOKIE_DOMAIN"`

	// CSRFAuthKey is the authentication key. It must be 32-bytes and can be
	// generated with tools/gen-secret. The value's should be base64 encoded.
	CSRFAuthKey envconfig.Base64Bytes `env:"CSRF_AUTH_KEY,required"`

	// Application Config
	ServerName string `env:"SERVER_NAME,default=Diagnosis Verification Server"`

	// Issue is configuration specific to the code issue APIs.
	Issue IssueAPIVars

	// If Dev mode is true, cookies aren't required to be sent over secure channels.
	// This includes CSRF protection base cookie. You want this false in production (the default).
	DevMode bool `env:"DEV_MODE"`

	// If MaintenanceMode is true, the server is temporarily read-only and will not issue codes.
	MaintenanceMode bool `env:"MAINTENANCE_MODE"`

	// Rate limiting configuration
	RateLimit ratelimit.Config
}

// NewServerConfig initializes and validates a ServerConfig struct.
func NewServerConfig(ctx context.Context) (*ServerConfig, error) {
	var config ServerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}

	// Parse system notice - do this once since it's displayed on every page.
	if v := project.TrimSpace(config.SystemNotice); v != "" {
		raw := blackfriday.Run([]byte(v))
		config.systemNotice = string(bluemonday.UGCPolicy().SanitizeBytes(raw))
	}

	// For the, when inserting into the javascript, gets escaped and becomes unusable.
	config.Firebase.DatabaseURL = strings.ReplaceAll(config.Firebase.DatabaseURL, "https://", "")
	return &config, nil
}

func (c *ServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.SessionDuration, "SESSION_DURATION"},
		{c.RevokeCheckPeriod, "REVOKE_CHECK_DURATION"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	if err := c.Issue.Validate(); err != nil {
		return fmt.Errorf("failed to validate issue API configuration: %w", err)
	}

	return nil
}

func (c *ServerConfig) IssueConfig() *IssueAPIVars {
	return &c.Issue
}

func (c *ServerConfig) GetRateLimitConfig() *ratelimit.Config {
	return &c.RateLimit
}

// The server module doesn't handle self report.
func (c *ServerConfig) GetUserReportTimeout() *time.Duration {
	return nil
}

func (c *ServerConfig) GetFeatureConfig() *FeatureConfig {
	return &c.Features
}

func (c *ServerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}

func (c *ServerConfig) IsMaintenanceMode() bool {
	return c.MaintenanceMode
}

func (c *ServerConfig) GetAuthenticatedSMSFailClosed() bool {
	return c.SMSSigning.FailClosed
}

func (c *ServerConfig) ParsedSystemNotice() string {
	return c.systemNotice
}

// FirebaseConfig represents configuration specific to firebase auth.
type FirebaseConfig struct {
	APIKey          string `env:"FIREBASE_API_KEY,required"`
	AuthDomain      string `env:"FIREBASE_AUTH_DOMAIN,required"`
	DatabaseURL     string `env:"FIREBASE_DATABASE_URL,required"`
	ProjectID       string `env:"FIREBASE_PROJECT_ID,required"`
	StorageBucket   string `env:"FIREBASE_STORAGE_BUCKET,required"`
	MessageSenderID string `env:"FIREBASE_MESSAGE_SENDER_ID,required"`
	AppID           string `env:"FIREBASE_APP_ID,required"`
	MeasurementID   string `env:"FIREBASE_MEASUREMENT_ID,required"`

	TermsOfServiceURL string `env:"FIREBASE_TERMS_OF_SERVICE_URL"`
	PrivacyPolicyURL  string `env:"FIREBASE_PRIVACY_POLICY_URL"`
}

// FirebaseConfig returns the firebase SDK config based on the local env config.
func (c *ServerConfig) FirebaseConfig() *firebase.Config {
	return &firebase.Config{
		DatabaseURL:   c.Firebase.DatabaseURL,
		ProjectID:     c.Firebase.ProjectID,
		StorageBucket: c.Firebase.StorageBucket,
	}
}

// Base64ByteSlice is a slice of base64-encoded strings that we want to convert
// to bytes.
type Base64ByteSlice []envconfig.Base64Bytes

// AsBytes returns the value as a slice of bytes instead of its main type.
func (c Base64ByteSlice) AsBytes() [][]byte {
	s := make([][]byte, len(c))
	for i, v := range c {
		s[i] = []byte(v)
	}
	return s
}
