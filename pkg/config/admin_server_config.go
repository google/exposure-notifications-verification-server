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
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

var _ IssueAPIConfig = (*AdminAPIServerConfig)(nil)

// AdminAPIServerConfig represents the environment based config for the Admin API Server.
type AdminAPIServerConfig struct {
	Database      database.Config
	Observability observability.Config
	Cache         cache.Config

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// Rate limiting configuration
	RateLimit ratelimit.Config

	Port                string        `env:"PORT,default=8080"`
	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`

	CollisionRetryCount uint          `env:"COLLISION_RETRY_COUNT,default=6"`
	AllowedSymptomAge   time.Duration `env:"ALLOWED_PAST_SYMPTOM_DAYS,default=336h"` // 336h is 14 days.
	EnforceRealmQuotas  bool          `env:"ENFORCE_REALM_QUOTAS, default=true"`

	// For EN Express, the link will be
	// https://[realm-region].[ENX_REDIRECT_DOMAIN]/v?c=[longcode]
	// This repository contains a redirect service that can be used for this purpose.
	ENExpressRedirectDomain string `env:"ENX_REDIRECT_DOMAIN"`
}

// NewAdminAPIServerConfig returns the environment config for the Admin API server.
// Only needs to be called once per instance, but may be called multiple times.
func NewAdminAPIServerConfig(ctx context.Context) (*AdminAPIServerConfig, error) {
	var config AdminAPIServerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *AdminAPIServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.APIKeyCacheDuration, "API_KEY_CACHE_DURATION"},
		{c.AllowedSymptomAge, "ALLOWED_PAST_SYMPTOM_DAYS"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	c.ENExpressRedirectDomain = strings.ToLower(c.ENExpressRedirectDomain)

	return nil
}

func (c *AdminAPIServerConfig) GetENXRedirectDomain() string {
	return c.ENExpressRedirectDomain
}

func (c *AdminAPIServerConfig) GetCollisionRetryCount() uint {
	return c.CollisionRetryCount
}

func (c *AdminAPIServerConfig) GetAllowedSymptomAge() time.Duration {
	return c.AllowedSymptomAge
}

func (c *AdminAPIServerConfig) GetEnforceRealmQuotas() bool {
	return c.EnforceRealmQuotas
}

func (c *AdminAPIServerConfig) GetRateLimitConfig() *ratelimit.Config {
	return &c.RateLimit
}

func (c *AdminAPIServerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
