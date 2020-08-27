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
	"sync"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// APIServerConfig represnets the environment based configuration for the API server.
type APIServerConfig struct {
	Database      database.Config
	Observability observability.Config
	Cache         cache.Config `env:",prefix=CACHE_"`

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	Port string `env:"PORT,default=8080"`

	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`

	// Verification Token Config
	VerificationTokenDuration time.Duration `env:"VERIFICATION_TOKEN_DURATION,default=24h"`

	// Token signing
	TokenSigning TokenSigningConfig

	// Certificate signing
	CertificateSigning CertificateSigningConfig

	// Rate limiting configuration
	RateLimit ratelimit.Config

	// cached allowed public keys
	allowedTokenPublicKeys map[string]string
	mu                     sync.RWMutex
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

// AllowedTokenPublicKeys returns a map of 'kid' to the KMS KeyID reference.
// This represents the keys that are allowed to be used to verify tokens,
// the TokenSigningKey/TokenSigningKeyID.
func (c *APIServerConfig) AllowedTokenPublicKeys() map[string]string {
	result, err := func() (map[string]string, error) {
		c.mu.RLock()
		defer c.mu.RUnlock()
		if len(c.allowedTokenPublicKeys) > 0 {
			return c.allowedTokenPublicKeys, nil
		}
		return nil, fmt.Errorf("missing")
	}()
	if err == nil {
		return result
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// handle race condition that could occur between lock upgrade.
	if len(c.allowedTokenPublicKeys) != 0 {
		return c.allowedTokenPublicKeys
	}

	c.allowedTokenPublicKeys = make(map[string]string, len(c.TokenSigning.TokenSigningKeyIDs))

	for i, kid := range c.TokenSigning.TokenSigningKeyIDs {
		c.allowedTokenPublicKeys[kid] = c.TokenSigning.TokenSigningKeys[i]
	}
	return c.allowedTokenPublicKeys
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

	return nil
}

func (c *APIServerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
