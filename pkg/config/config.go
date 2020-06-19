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

// Package config defines the environment baased configuration for this server.
package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

// New returns the environment config for the server. Only needs to be called once
// per instance, but may be called multiple times.
func New(ctx context.Context) (*Config, error) {
	return NewWith(ctx, envconfig.OsLookuper())
}

// NewWith creates a new config with the given lookuper for parsing config.
func NewWith(ctx context.Context, l envconfig.Lookuper) (*Config, error) {
	// Build a list of mutators. This list will grow as we initialize more of the
	// configuration, such as the secret manager.
	var mutatorFuncs []envconfig.MutatorFunc

	{
		// Load the secret manager configuration first - this needs to be loaded first
		// because other processors may need secrets.
		var smConfig secrets.Config
		if err := envconfig.ProcessWith(ctx, &smConfig, l); err != nil {
			return nil, fmt.Errorf("unable to process secret configuration: %w", err)
		}

		sm, err := secrets.SecretManagerFor(ctx, smConfig.SecretManagerType)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to secret manager: %w", err)
		}

		// Enable caching, if a TTL was provided.
		if ttl := smConfig.SecretCacheTTL; ttl > 0 {
			sm, err = secrets.WrapCacher(ctx, sm, ttl)
			if err != nil {
				return nil, fmt.Errorf("unable to create secret manager cache: %w", err)
			}
		}

		// Update the mutators to process secrets.
		mutatorFuncs = append(mutatorFuncs, secrets.Resolver(sm, &smConfig))
	}

	// Parse the main configuration.
	var config Config
	if err := envconfig.ProcessWith(ctx, &config, l, mutatorFuncs...); err != nil {
		return nil, err
	}

	// For the, when inserting into the javascript, gets escaped and becomes unusable.
	config.Firebase.DatabaseURL = strings.ReplaceAll(config.Firebase.DatabaseURL, "https://", "")

	return &config, nil
}

// Config represents the environment based config for the server.
type Config struct {
	Firebase FirebaseConfig
	Database database.Config

	// Login Config
	SessionCookieDuration time.Duration `env:"SESSION_DURATION,default=24h"`
	RevokeCheckPeriod     time.Duration `env:"REVOKE_CHECK_DURATION,default=5m"`

	// Application Config
	ServerName          string        `env:"SERVER_NAME,default=Diagnosis Verification Server"`
	CodeDuration        time.Duration `env:"CODE_DURATION,default=1h"`
	CodeDigits          int           `env:"CODE_DIGITS,default=8"`
	ColissionRetryCount int           `env:"COLISSION_RETRY_COUNT,default=6"`
	AllowedTestAge      time.Duration `env:"ALLOWRD_PAST_TEST_DAYS,default=336h"` // 336h is 14 days.
	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`
	RateLimit           int64         `env:"RATE_LIMIT,default=60"`

	// Verification Token Config
	VerificationTokenDuration time.Duration `env:"VERIFICATION_TOKEN_DURATION,default=24h"`
	TokenSigningKey           string        `env:"TOKEN_SIGNING_KEY,required"`
	TokenIssuer               string        `env:"TOKEN_ISSUER,default=diagnosis-verification-example"`

	// Verification certificate config
	PublicKeyCacheDuration time.Duration `env:"PUBLIC_KEY_CACHE_DURATION,default=15m"`
	CertificateSigningKey  string        `env:"CERTIFICATE_SIGNING_KEY,required"`
	CertificateIssuer      string        `env:"CERTIFICATE_ISSUER,default=diagnosis-verification-example"`
	CertificateAudience    string        `env:"CERTIFICATE_AUDIENCE,default=exposure-notifications-server"`
	CertificateDuration    time.Duration `env:"CERTIFICATE_DURATION,default=15m"`

	// Cleanup config
	CleanupPeriod           time.Duration `env:"CLEANUP_PERIOD,default=15m"`
	DisabledUserMaxAge      time.Duration `env:"DIABLED_USER_MAX_AGE,default=336h"`
	VerificationCodeMaxAge  time.Duration `env:"VERIFICATION_CODE_MAX_AGE,default=24h"`
	VerificationTokenMaxAge time.Duration `env:"VERIFICATION_TOKEN_MAX_AGE,default=24h"`

	AssetsPath string `env:"ASSETS_PATH,default=./cmd/server/assets"`
}

// FirebaseConfig represents configuration specific to firebase auth.
type FirebaseConfig struct {
	APIKey          string `env:"FIREBASE_API_KEY,required"`
	AuthDomain      string `env:"FIREBASE_AUTH_DOMAIN,required"`
	DatabaseURL     string `env:"FIREBASE_DATABASE_URL,required"`
	ProjectID       string `env:"FIREBASE_PROJECT_ID,required"`
	StorageBucket   string `env:"FIREBASE_STORAGE_BUCKET,required"`
	MessageSenderID string `env:"FIREBASE_MESSAGE_SENDER_ID,required"`
	AppID           string `env:"FIREBASE_APP_ID,required"`
	MeasurementID   string `env:"FIREBASE_MEASUREMENT_ID,required"`
}

// FirebaseConfig returns the firebase SDK config based on the local env config.
func (c *Config) FirebaseConfig() *firebase.Config {
	return &firebase.Config{
		DatabaseURL:   c.Firebase.DatabaseURL,
		ProjectID:     c.Firebase.ProjectID,
		StorageBucket: c.Firebase.StorageBucket,
	}
}
