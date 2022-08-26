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

package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"

	"github.com/google/exposure-notifications-server/pkg/logging"
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

	// CookieDomain is the domain for which cookie should be valid.
	CookieDomain string `env:"COOKIE_DOMAIN"`

	// Application Config
	ServerName string `env:"SERVER_NAME,default=Exposure Notifications Verification Server"`

	// ServerEndpoint is the custom endpoint for the server (scheme + host [+
	// port]). If empty, the system will attempt to guess based on the request.
	ServerEndpoint string `env:"SERVER_ENDPOINT"`

	// Issue is configuration specific to the code issue APIs.
	Issue IssueAPIVars

	// If Dev mode is true, cookies aren't required to be sent over secure channels.
	// This includes CSRF protection base cookie. You want this false in production (the default).
	DevMode bool `env:"DEV_MODE"`

	// If MaintenanceMode is true, the server is temporarily read-only and will not issue codes.
	MaintenanceMode bool `env:"MAINTENANCE_MODE"`

	// MinRealmsForSystemStatistics gives a minimum threshold for displaying system
	// admin level statistics
	MinRealmsForSystemStatistics uint `env:"MIN_REALMS_FOR_SYSTEM_STATS, default=2"`

	// Rate limiting configuration
	RateLimit ratelimit.Config
}

// NewServerConfig initializes and validates a ServerConfig struct.
func NewServerConfig(ctx context.Context) (*ServerConfig, error) {
	var c ServerConfig
	if err := ProcessWith(ctx, &c, envconfig.OsLookuper()); err != nil {
		return nil, err
	}

	if err := c.Process(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

// Process processes the config. This is an internal API, but is public for
// cross-package sharing.
func (c *ServerConfig) Process(ctx context.Context) error {
	// Deprecations
	logger := logging.FromContext(ctx).Named("config.ServerConfig")
	if v := os.Getenv("CSRF_AUTH_KEY"); v != "" {
		logger.Warnw("CSRF_AUTH_KEY is no longer used, please remove it from your configuration")
	}

	// Append maintenance mode text to the system notice.
	if c.MaintenanceMode {
		existing := c.SystemNotice
		c.SystemNotice = `The server is undergoing maintenance and is read-only. Requests to issue new codes will fail.`
		if existing != "" {
			c.SystemNotice = c.SystemNotice + " " + existing
		}
	}

	// Parse system notice - do this once since it's displayed on every page.
	if v := project.TrimSpace(c.SystemNotice); v != "" {
		raw := blackfriday.Run([]byte(strings.TrimSpace(v)))
		c.systemNotice = string(bluemonday.UGCPolicy().SanitizeBytes(raw))
	}

	// For the, when inserting into the javascript, gets escaped and becomes unusable.
	c.Firebase.DatabaseURL = strings.ReplaceAll(c.Firebase.DatabaseURL, "https://", "")
	return nil
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

	if c.MinRealmsForSystemStatistics < 2 {
		return fmt.Errorf("MIN_REALMS_FOR_SYSTEM_STATS cannot be set lower than 2")
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
