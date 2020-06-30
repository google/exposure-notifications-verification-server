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

// Package config defines the environment baased configuration for this project.
// Each server has a unique config type.
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/pkg/secrets"

	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

// Validatable indicates that a type can be validated.
type Validatable interface {
	Validate() error
}

// Helper func for handling env vars
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// VerificationCodeIssuerConfig defines required config fields common to a server that issues verification codes
type VerificationCodeIssuerConfig interface {
	AllowedTestAge() time.Duration
	CodeDigits() uint
	CodeDuration() time.Duration
	CollisionRetryCount() uint
}

// VerificationCodeIssuerConfigImp implementation for ease of use
type VerificationCodeIssuerConfigImp struct{}

// AllowedTestAge with default of 336h (14 days), -1 implies error
func (c *VerificationCodeIssuerConfigImp) AllowedTestAge() time.Duration {
	val, err := time.ParseDuration(getEnv("ALLOWED_PAST_TEST_DAYS", "336h"))
	if err != nil {
		return -1
	}
	return val
}

// CodeDigits with default of 8, 0 implies error
func (c *VerificationCodeIssuerConfigImp) CodeDigits() uint {
	val, err := strconv.ParseUint(getEnv("CODE_DIGITS", "8"), 10, 32)
	if err != nil {
		return 0
	}
	return uint(val)
}

// CodeDuration with default of 1h, -1 implies error
func (c *VerificationCodeIssuerConfigImp) CodeDuration() time.Duration {
	val, err := time.ParseDuration(getEnv("CODE_DURATION", "1h"))
	if err != nil {
		return -1
	}
	return val
}

// CollisionRetryCount with default of 6, 0 implies error
func (c *VerificationCodeIssuerConfigImp) CollisionRetryCount() uint {
	val, err := strconv.ParseUint(getEnv("COLISSION_RETRY_COUNT", "6"), 10, 32)
	if err != nil {
		return 0
	}
	return uint(val)
}

// ProcessWith creates a new config with the given lookuper for parsing config.
func ProcessWith(ctx context.Context, spec Validatable, l envconfig.Lookuper) error {
	// Build a list of mutators. This list will grow as we initialize more of the
	// configuration, such as the secret manager.
	var mutatorFuncs []envconfig.MutatorFunc

	{
		// Load the secret manager configuration first - this needs to be loaded first
		// because other processors may need secrets.
		var smConfig secrets.Config
		if err := envconfig.ProcessWith(ctx, &smConfig, l); err != nil {
			return fmt.Errorf("unable to process secret configuration: %w", err)
		}

		sm, err := secrets.SecretManagerFor(ctx, smConfig.SecretManagerType)
		if err != nil {
			return fmt.Errorf("unable to connect to secret manager: %w", err)
		}

		// Enable caching, if a TTL was provided.
		if ttl := smConfig.SecretCacheTTL; ttl > 0 {
			sm, err = secrets.WrapCacher(ctx, sm, ttl)
			if err != nil {
				return fmt.Errorf("unable to create secret manager cache: %w", err)
			}
		}

		// Update the mutators to process secrets.
		mutatorFuncs = append(mutatorFuncs, secrets.Resolver(sm, &smConfig))
	}

	// Parse the main configuration.
	if err := envconfig.ProcessWith(ctx, spec, l, mutatorFuncs...); err != nil {
		return err
	}

	if err := spec.Validate(); err != nil {
		return err
	}

	return nil
}

func checkPositiveDuration(d time.Duration, name string) error {
	if d < 0 {
		return fmt.Errorf("%v must be a positive duration, got: %v", name, d)
	}
	return nil
}

func checkNonzero(v uint, name string) error {
	if v == 0 {
		return fmt.Errorf("%v must be a nonzero unsigned integer, got: %v", name, v)
	}
	return nil
}
