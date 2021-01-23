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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// CleanupConfig represents the environment based configuration for the Cleanup server.
type CleanupConfig struct {
	Database      database.Config
	Observability observability.Config
	Features      FeatureConfig

	// TokenSigning is the token signing configuration to purge old keys in the
	// key manager when they are cleaned.
	TokenSigning TokenSigningConfig

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// Port is the port on which to bind.
	Port string `env:"PORT,default=8080"`

	// Cleanup config
	AuditEntryMaxAge    time.Duration `env:"AUDIT_ENTRY_MAX_AGE, default=720h"`
	AuthorizedAppMaxAge time.Duration `env:"AUTHORIZED_APP_MAX_AGE, default=336h"`
	CleanupMinPeriod    time.Duration `env:"CLEANUP_MIN_PERIOD, default=15m"`
	// KeyServerStatsMaxAge is the maximum amount of time to retain key-server stats.
	KeyServerStatsMaxAge time.Duration `env:"KEY_SERVER_STATS_MAX_AGE, default=720h"`
	MobileAppMaxAge      time.Duration `env:"MOBILE_APP_MAX_AGE, default=168h"`

	// SigningTokenKeyMaxAge is the maximum amount of time that a rotated signing
	// token key should remain unpurged.
	SigningTokenKeyMaxAge time.Duration `env:"SIGNING_TOKEN_KEY_MAX_AGE, default=36h"`

	// VerificationSigningKeyMaxAge is the maximum amount of time that an already soft
	// delted SigningKey will be kept in the database before being purged.
	VerificationSigningKeyMaxAge time.Duration `env:"VERIFICATION_SIGNING_KEY_MAX_AGE, default=36h"`

	UserPurgeMaxAge time.Duration `env:"USER_PURGE_MAX_AGE, default=720h"`
	// VerificationCodeMaxAge is the period in which the full code should be available.
	// After this time it will be recycled. The code will be zeroed out, but its status persist.
	VerificationCodeMaxAge time.Duration `env:"VERIFICATION_CODE_MAX_AGE, default=48h"`
	// VerificationCodeStatusMaxAge is the time after which, even the status of the code will be deleted
	// and the entry will be purged. This value should be greater than VerificationCodeMaxAge
	VerificationCodeStatusMaxAge time.Duration `env:"VERIFICATION_CODE_STATUS_MAX_AGE, default=336h"`
	VerificationTokenMaxAge      time.Duration `env:"VERIFICATION_TOKEN_MAX_AGE, default=24h"`
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
		{c.CleanupMinPeriod, "CLEANUP_MIN_PERIOD"},
		{c.VerificationCodeMaxAge, "VERIFICATION_CODE_MAX_AGE"},
		{c.VerificationCodeStatusMaxAge, "VERIFICATION_CODE_STATUS_MAX_AGE"},
		{c.VerificationTokenMaxAge, "VERIFICATION_TOKEN_MAX_AGE"},
		{c.AuditEntryMaxAge, "AUDIT_ENTRY_MAX_AGE"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	// Audit entries need to persist for at least 7 days. The default is 30d ays.
	if c.AuditEntryMaxAge < 7*24*time.Hour {
		return fmt.Errorf("AUDIT_ENTRY_MAX_AGE must be at least 7 days")
	}

	if c.VerificationCodeStatusMaxAge < c.VerificationCodeMaxAge {
		return fmt.Errorf("the code status %q is expected to live longer than the life of the code %q",
			c.VerificationCodeStatusMaxAge.String(), c.VerificationCodeMaxAge.String())
	}

	return nil
}

func (c *CleanupConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
