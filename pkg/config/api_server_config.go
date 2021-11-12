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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

var _ IssueAPIConfig = (*APIServerConfig)(nil)

// APIServerConfig represnets the environment based configuration for the API server.
type APIServerConfig struct {
	Database      database.Config
	Observability observability.Config
	Cache         cache.Config
	Features      FeatureConfig

	// SMSSigning defines the SMS signing configuration.
	SMSSigning SMSSigningConfig

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// If MaintenanceMode is true, the server is temporarily read-only and will not issue codes.
	MaintenanceMode bool `env:"MAINTENANCE_MODE"`

	Port              string `env:"PORT,default=8080"`
	ChaffMaxLatencyMs uint64 `env:"CHAFF_MAX_LATENCY_MS, default=1000"`

	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`

	// Verification Token Config
	VerificationTokenDuration time.Duration `env:"VERIFICATION_TOKEN_DURATION,default=24h"`

	// Token signing
	TokenSigning TokenSigningConfig

	// Certificate signing
	CertificateSigning CertificateSigningConfig

	// Rate limiting configuration
	RateLimit ratelimit.Config

	// variables for Issue API
	Issue IssueAPIVars
}

// NewAPIServerConfig returns the environment config for the API server.
// Only needs to be called once per instance, but may be called multiple times.
func NewAPIServerConfig(ctx context.Context) (*APIServerConfig, error) {
	var config APIServerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *APIServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.APIKeyCacheDuration, "API_KEY_CACHE_DURATION"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	if err := c.TokenSigning.Validate(); err != nil {
		return fmt.Errorf("failed to validate signing token configuration: %w", err)
	}

	if err := c.Issue.Validate(); err != nil {
		return fmt.Errorf("failed to validate issue API configuration: %w", err)
	}

	return nil
}

func (c *APIServerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}

func (c *APIServerConfig) IssueConfig() *IssueAPIVars {
	return &c.Issue
}

func (c *APIServerConfig) GetRateLimitConfig() *ratelimit.Config {
	return &c.RateLimit
}

func (c *APIServerConfig) GetFeatureConfig() *FeatureConfig {
	return &c.Features
}

func (c *APIServerConfig) GetAuthenticatedSMSFailClosed() bool {
	return c.SMSSigning.FailClosed
}

func (c *APIServerConfig) IsMaintenanceMode() bool {
	return c.MaintenanceMode
}
