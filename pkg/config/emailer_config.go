// Copyright 2022 the Exposure Notifications Verification Server authors
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
	"net/mail"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"

	"github.com/sethvargo/go-envconfig"
)

// EmailerConfig represents the environment-based configuration for the emailer
// service.
type EmailerConfig struct {
	Database      database.Config
	Observability observability.Config
	Features      FeatureConfig
	Secrets       secrets.Config

	// Port is the port upon which to bind.
	Port string `env:"PORT, default=8080"`

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// MinTTL is the minimum amount of time that must elapse between attempting
	// emailer events. This is used to control whether emails are actually sent at
	// the controller layer, independent of being invoked via a scheduler.
	MinTTL time.Duration `env:"MIN_TTL, default=4h"`

	// FromAddress is the address from which to send emails. This must be an
	// address that resides in the Google Workspace domain. It can be of the
	// format "user@example.com". The recommended value is
	// "no-reply@your-server.com".
	FromAddress string `env:"FROM_ADDRESS"`

	// MailDomain is the domain from which to send email. It should be just the
	// domain (no https:// or port).
	MailDomain string `env:"MAIL_DOMAIN"`

	// EmailCC and EmailBCC is the address to CC and BCC (respectively) on all
	// emails.
	CCAddresses  []string `env:"CC_ADDRESSES"`
	BCCAddresses []string `env:"BCC_ADDRESSES"`

	// ServerEndpoint is the URL to the main verification server component - the
	// UI server. It should be the full URL with no trailing slash.
	ServerEndpoint string `env:"SERVER_ENDPOINT"`

	// SMTPRelayHost and SMTPRelayPort are the URLs for the SMTP server. The
	// default values should be appropriate for most situations.
	SMTPRelayHost string `env:"SMTP_RELAY_HOST, default=smtp-relay.gmail.com"`
	SMTPRelayPort string `env:"SMTP_RELAY_PORT, default=587"`

	// SMSIgnoredErrorCodes is a list of SMS error codes to ignore.
	//
	// 30003 - Phone is off
	// 30004 - User blocked receiving messages from this number
	// 30005 - Invalid phone number
	// 30006 - Landline error
	SMSIgnoredErrorCodes []string `env:"SMS_IGNORED_ERROR_CODES, default=30003,30004,30005,30006"`

	// SMSErrorsEmailThreshold is the number of SMS errors in a given 24 hour UTC
	// period at which email alerts will begin being generated. This applies to
	// all realms on the system.
	SMSErrorsEmailThreshold int64 `env:"SMS_ERRORS_EMAIL_THRESHOLD, default=50"`
}

// NewEmailerConfig returns the config for the emailer service.
func NewEmailerConfig(ctx context.Context) (*EmailerConfig, error) {
	var config EmailerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *EmailerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
		Min  time.Duration
	}{
		{c.MinTTL, "MIN_TTL", 0},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	if from := c.FromAddress; from != "" {
		if _, err := mail.ParseAddress(from); err != nil {
			return fmt.Errorf("invalid FROM_ADDRESS: %w", err)
		}
	}

	for _, addr := range c.CCAddresses {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("invalid CC_ADDRESSES %q: %w", addr, err)
		}
	}

	for _, addr := range c.BCCAddresses {
		if _, err := mail.ParseAddress(addr); err != nil {
			return fmt.Errorf("invalid BCC_ADDRESSES %q: %w", addr, err)
		}
	}

	return nil
}

func (c *EmailerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
