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

	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/sethvargo/go-envconfig"
)

var _ IssueAPIConfig = (*RedirectConfig)(nil)

// RedirectConfig represents the environment based config for the redirect server.
type RedirectConfig struct {
	Database      database.Config
	Observability observability.Config
	Cache         cache.Config
	Features      FeatureConfig

	Port string `env:"PORT, default=8080"`

	AppCacheTTL time.Duration `env:"APP_CACHE_TTL, default=5m"`

	// If Dev mode is true, extended logging is enabled and template
	// auto-reload is enabled.
	DevMode bool `env:"DEV_MODE"`

	SessionDuration    time.Duration `env:"SESSION_DURATION, default=1h"`
	SessionIdleTimeout time.Duration `env:"SESSION_IDLE_TIMEOUT, default=20m"`

	// CookieKeys is a slice of bytes. The first is 64 bytes, the second is 32.
	// They should be base64-encoded.
	CookieKeys []envconfig.Base64Bytes `env:"COOKIE_KEYS"`

	// Issue config is pulled in for the ENX_REDIRECT_DOMAIN
	Issue IssueAPIVars

	// Rate limiting configuration
	RateLimit ratelimit.Config

	// SMSSigning defines the SMS signing configuration.
	SMSSigning SMSSigningConfig

	// If MaintenanceMode is true, the server is temporarily read-only and will not issue codes.
	MaintenanceMode bool `env:"MAINTENANCE_MODE"`

	// A map of hostnames to redirect to ens:// and a mapping to the region.
	// For example to redirect
	//   region.example.com to region US-AA
	//   otherregion.example.com to region US-BB
	// all matched hostnames are redirected to
	// "ens://"
	// The append region is added to the end
	// "US-AA,US-BB"
	//
	// The config for this is passed as a map, example:
	// HOSTNAME_TO_REGION="region.example.com:US-AA,otherregion.example.com:US-BB"
	HostnameConfig map[string]string `env:"HOSTNAME_TO_REGION"`
}

// NewRedirectConfig initializes and validates a RedirectConfig struct.
func NewRedirectConfig(ctx context.Context) (*RedirectConfig, error) {
	var config RedirectConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *RedirectConfig) IsMaintenanceMode() bool {
	return c.MaintenanceMode
}

func (c *RedirectConfig) IssueConfig() *IssueAPIVars {
	return &c.Issue
}

func (c *RedirectConfig) GetRateLimitConfig() *ratelimit.Config {
	return &c.RateLimit
}

func (c *RedirectConfig) GetFeatureConfig() *FeatureConfig {
	return &c.Features
}

func (c *RedirectConfig) GetAuthenticatedSMSFailClosed() bool {
	return c.SMSSigning.FailClosed
}

func (c *RedirectConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}

func (c *RedirectConfig) DatabaseConfig() *database.Config {
	return &c.Database
}

// HostnameToRegion returns a normalized map of the HOSTNAME_TO_REGION config value.
// Hostnames (key) are lowercased
// Regions (value) are uppercased
func (c *RedirectConfig) HostnameToRegion() (map[string]string, error) {
	hostnameToRegion := make(map[string]string, len(c.HostnameConfig))
	for hostname, region := range c.HostnameConfig {
		if hostname == "" {
			return nil, fmt.Errorf("hostname empty for region value: %v", region)
		}
		if region == "" {
			return nil, fmt.Errorf("hostname %v is missing region", hostname)
		}
		hostnameToRegion[strings.ToLower(hostname)] = strings.ToUpper(region)
	}
	return hostnameToRegion, nil
}
