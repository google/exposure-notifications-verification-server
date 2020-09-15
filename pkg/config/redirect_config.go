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

	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// RedirectConfig represents the environment based config for the redirect server.
type RedirectConfig struct {
	Observability observability.Config

	Port string `env:"PORT,default=8080"`

	AssetsPath string `env:"KO_DATA_PATH"`

	// If Dev mode is true, cookies aren't required to be sent over secure channels.
	// This includes CSRF protection base cookie. You want this false in production (the default).
	DevMode bool `env:"DEV_MODE"`

	RedirectFrom []string `env:"REDIRECT_FROM"`
	RedirectTo   []string `env:"REDIRECT_TO"`

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
