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

	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

// APIServerConfig represnets the environment based configuration for the API server.
type APIServerConfig struct {
	Database database.Config

	Port      int    `env:"PORT,default=8080"`
	RateLimit uint64 `env:"RATE_LIMIT,default=60"`

	// Verification Code Issuer Config
	*VerificationCodeIssuerConfigImp

	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`

	// Currently this does not easily support rotation. TODO(mikehelmick) - add support.
	VerificationTokenDuration time.Duration `env:"VERIFICATION_TOKEN_DURATION,default=24h"`
	TokenSigningKey           string        `env:"TOKEN_SIGNING_KEY,required"`
	TokenSigningKeyID         string        `env:"TOKEN_SIGNING_KEY_ID,default=v1"`
	TokenIssuer               string        `env:"TOKEN_ISSUER,default=diagnosis-verification-example"`

	// Verification certificate config
	PublicKeyCacheDuration  time.Duration `env:"PUBLIC_KEY_CACHE_DURATION,default=15m"`
	CertificateSigningKey   string        `env:"CERTIFICATE_SIGNING_KEY,required"`
	CertificateSigningKeyID string        `env:"CERTIFICATE_SIGNING_KEY_ID,default=v1"`
	CertificateIssuer       string        `env:"CERTIFICATE_ISSUER,default=diagnosis-verification-example"`
	CertificateAudience     string        `env:"CERTIFICATE_AUDIENCE,default=exposure-notifications-server"`
	CertificateDuration     time.Duration `env:"CERTIFICATE_DURATION,default=15m"`
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
	timeFields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.APIKeyCacheDuration, "API_KEY_CACHE_DURATION"},
		{c.PublicKeyCacheDuration, "PUBLIC_KEY_CACHE_DURATION"},
		{c.CodeDuration(), "CODE_DURATION"},
		{c.AllowedTestAge(), "ALLOWED_PAST_TEST_DAYS"},
	}

	uintFields := []struct {
		Var  uint
		Name string
	}{
		{c.CodeDigits(), "CODE_DIGITS"},
		{c.CollisionRetryCount(), "COLISSION_RETRY_COUNT"},
	}

	for _, f := range timeFields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	for _, f := range uintFields {
		if err := checkNonzero(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
}
