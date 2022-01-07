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

	// ServerEndpoint is the URL to the main verification server component - the
	// UI server. It should be the full URL with no trailing slash.
	ServerEndpoint string `env:"SERVER_ENDPOINT"`

	// SMTPRelayHost and SMTPRelayPort are the URLs for the SMTP server. The
	// default values should be appropriate for most situations.
	SMTPRelayHost string `env:"SMTP_RELAY_HOST, default=smtp-relay.gmail.com"`
	SMTPRelayPort string `env:"SMTP_RELAY_PORT, default=587"`
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

	return nil
}

func (c *EmailerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
