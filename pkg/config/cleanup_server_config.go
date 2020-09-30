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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// CleanupConfig represents the environment based configuration for the Cleanup server.
type CleanupConfig struct {
	Database      database.Config
	Observability observability.Config

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	Port string `env:"PORT,default=8080"`

	RateLimit uint64 `env:"RATE_LIMIT,default=60"`

	// Cleanup config
	CleanupPeriod           time.Duration `env:"CLEANUP_PERIOD,default=15m"`
	VerificationCodeMaxAge  time.Duration `env:"VERIFICATION_CODE_MAX_AGE,default=24h"`
	VerificationTokenMaxAge time.Duration `env:"VERIFICATION_TOKEN_MAX_AGE,default=24h"`
	MobileAppMaxAge         time.Duration `env:"MOBILE_APP_MAX_AGE,default=168h"`
	AuditEntryMaxAge        time.Duration `env:"AUDIT_ENTRY_MAX_AGE,default=720h"`
}

// NewCleanupConfig returns the environment config for the cleanup server.
// Only needs to be called once per instance, but may be called multiple times.
func NewCleanupConfig(ctx context.Context) (*CleanupConfig, error) {
	var config CleanupConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *CleanupConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.VerificationCodeMaxAge, "VERIFICATION_TOKEN_DURATION"},
		{c.CleanupPeriod, "CLEANUP_PERIOD"},
		{c.VerificationCodeMaxAge, "VERIFICATION_CODE_MAX_AGE"},
		{c.VerificationTokenMaxAge, "VERIFICATION_TOKEN_MAX_AGE"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
}

func (c *CleanupConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
