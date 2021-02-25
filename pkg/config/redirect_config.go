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
	"os"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/sethvargo/go-envconfig"
)

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
	logger := logging.FromContext(ctx).Named("RedirectConfig")

	var config RedirectConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}

	if v := os.Getenv("ASSETS_PATH"); v != "" {
		logger.Warnw("ASSETS_PATH is no longer used")
	}
	return &config, nil
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
