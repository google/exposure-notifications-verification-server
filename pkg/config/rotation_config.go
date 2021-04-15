// Copyright 2021 Google LLC
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

// RotationConfig represents the environment-based configuration for the
// rotation service.
type RotationConfig struct {
	Database      database.Config
	Observability observability.Config
	Features      FeatureConfig
	Secrets       secrets.Config

	// ProjectID is the Google Cloud project ID.
	ProjectID string `env:"PROJECT_ID, required"`

	// Port is the port upon which to bind.
	Port string `env:"PORT, default=8080"`

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// MinTTL is the minimum amount of time that must elapse between attempting
	// rotation events. This is used to control whether rotation is actually
	// attempted at the controller layer, independent of the data layer. In
	// effect, it rate limits the number of rotation requests.
	MinTTL time.Duration `env:"MIN_TTL, default=5m"`

	// TokenSigning is the token signing configuration. This defines the parent
	// key and common data like issuer, but the individual versions are controlled
	// by the database table.
	TokenSigning TokenSigningConfig

	// TokenSigningKeyMaxAge is the maximum age for a token signing key.
	TokenSigningKeyMaxAge time.Duration `env:"TOKEN_SIGNING_KEY_MAX_AGE, default=720h"` // 30 days

	// Verification rotation frequency.
	VerificationSigningKeyMaxAge time.Duration `env:"VERIFICATION_SIGNING_KEY_MAX_AGE, default=720h"` // 30 days
	// How long to wait to activate a new key after creation. This gives
	// the upstream key server time to import the new allowed public key.
	// A deactivated key will also be kept for this time period.
	VerificationActivationDelay time.Duration `env:"VERIFICATION_ACTIVATION_DELAY, default=1h"`
}

// NewRotationConfig returns the config for the rotation service.
func NewRotationConfig(ctx context.Context) (*RotationConfig, error) {
	var config RotationConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *RotationConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.TokenSigningKeyMaxAge, "TOKEN_SIGNING_KEY_MAX_AGE"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
}

func (c *RotationConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
