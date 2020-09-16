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

	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// RedirectConfig represents the environment based config for the redirect server.
type RedirectConfig struct {
	Observability observability.Config

	Port string `env:"PORT,default=8080"`

	AssetsPath string `env:"ASSETS_PATH,default=./cmd/enx-redirect/assets"`

	// If Dev mode is true, extended logging is enabled and tempalte
	// auto-reload is enabled.
	DevMode bool `env:"DEV_MODE"`

	// A list of prefixes to match.
	// "region.example.com,otherregion.example.com"
	// all prefixes are redirected to
	// "ens://"
	// The append region is added to the end
	// "US-AA,US-BB"
	//
	// As an example, if the redirect service receives a request like
	//  https://region.example.com/v?c=1234
	// Will result in a redict to (based on the above configuration)
	//  ens://v?c=1234&r=US-AA
	Hostnames   []string `env:"HOSTNAME"`
	RegionCodes []string `env:"REGION_CODE"`

	// Rate limiting configuration
	RateLimit ratelimit.Config
}

// NewRedirectConfig initializes and validates a RedirectConfig struct.
func NewRedirectConfig(ctx context.Context) (*RedirectConfig, error) {
	var config RedirectConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *RedirectConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}

func (c *RedirectConfig) HostnameToRegion() (map[string]string, error) {
	if len(c.Hostnames) != len(c.RegionCodes) {
		return nil, fmt.Errorf("HOSTNAME and REGION_CODE must be lists of the same length")
	}

	hostnameToRegion := make(map[string]string, len(c.Hostnames))
	for i, prefix := range c.Hostnames {
		hostnameToRegion[prefix] = strings.ToUpper(c.RegionCodes[i])
	}
	return hostnameToRegion, nil
}
