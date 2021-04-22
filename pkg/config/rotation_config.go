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
	MinTTL time.Duration `env:"MIN_TTL, default=1m"`

	// SecretsParent is the parent directory where secrets should be written. This
	// is only used when creating new secret versions. On Google Cloud, this
	// should be "projects/$PROJECT_ID/secrets".
	SecretsParent string `env:"SECRETS_PARENT, default=projects/$PROJECT_ID/secrets"`

	// SecretActivationTTL is the amount of time a secret should have been
	// "created" before being moved to the "active" state. This is used to ensure
	// a secret has fully propagated to all instances before being promoted to
	// active.
	//
	// SecretDestroyTTL is the amount of time a secret should be soft-deleted
	// before being purged in the upstream secret manager.
	SecretActivationTTL time.Duration `env:"SECRET_ACTIVATION_TTL, default=30m"`
	SecretDestroyTTL    time.Duration `env:"SECRET_DESTROY_TTL, default=24h"`

	// CookieKeyMinAge is the TTL at which a new cookie key will be created.
	// CookieKeyMaxAge is the TTL at which a cookie key is deleted.
	CookieKeyMinAge time.Duration `env:"COOKIE_KEY_MIN_AGE, default=720h"`  // 30d
	CookieKeyMaxAge time.Duration `env:"COOKIE_KEY_MAX_AGE, default=1488h"` // 60d + 2d

	// APIKeyDatabaseHMACKeyMinAge is the age at which to generate a new HMAC
	// key for HMACing API keys. Revoking an old HMAC key revokes all API keys
	// HMACed with that key, so existing values are kept.
	APIKeyDatabaseHMACKeyMinAge time.Duration `env:"API_KEY_DATABASE_HMAC_KEY_MIN_AGE, default=4320h"` // 180d

	// APIKeySignatureHMACKeyMinAge is the age at which to generate a new HMAC
	// key for signing API keys. Revoking an old signing key revokes all API keys
	// signed with that key, so existing values are kept.
	APIKeySignatureHMACKeyMinAge time.Duration `env:"API_KEY_SIGNATURE_HMAC_KEY_MIN_AGE, default=4320h"` // 180d

	// PhoneNumberDatabaseHMACKeyMinAge is the age at which to generate a new HMAC
	// key for HMACing phone numbers. PhoneNumberDatabaseHMACKeyMaxAge is the age
	// at which an HMAC key is deleted (which is 90 days after the last time the
	// key was used).
	PhoneNumberDatabaseHMACKeyMinAge time.Duration `env:"PHONE_NUMBER_DATABASE_HMAC_MIN_AGE, default=720h"`  // 30d
	PhoneNumberDatabaseHMACKeyMaxAge time.Duration `env:"PHONE_NUMBER_DATABASE_HMAC_MAX_AGE, default=2928h"` // 30d + 90d + 2d

	// VerificationCodeDatabaseHMACKeyMinAge is the age at which to generate a new
	// HMAC key for HMACing verification codes in the database.
	// VerificationCodeDatabaseHMACKeyMaxAge is the age at which the HMAC key can
	// be safely deleted.
	VerificationCodeDatabaseHMACKeyMinAge time.Duration `env:"VERIFICATION_CODE_DATABASE_HMAC_KEY_MIN_AGE, default=24h"`
	VerificationCodeDatabaseHMACKeyMaxAge time.Duration `env:"VERIFICATION_CODE_DATABASE_HMAC_KEY_MAX_AGE, default=72h"` // 2d + 1d

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
		Min  time.Duration
	}{
		{c.MinTTL, "MIN_TTL", 0},
		{c.SecretActivationTTL, "SECRET_ACTIVATION_TTL", 0},
		{c.SecretDestroyTTL, "SECRET_DESTROY_TTL", 0},
		{c.CookieKeyMinAge, "COOKIE_KEY_MIN_AGE", 0},
		{c.CookieKeyMaxAge, "COOKIE_KEY_MAX_AGE", 0},
		{c.APIKeyDatabaseHMACKeyMinAge, "API_KEY_DATABASE_HMAC_KEY_MIN_AGE", 0},
		{c.APIKeySignatureHMACKeyMinAge, "API_KEY_SIGNATURE_HMAC_KEY_MIN_AGE", 0},
		{c.PhoneNumberDatabaseHMACKeyMinAge, "PHONE_NUMBER_DATABASE_HMAC_MIN_AGE", 0},
		{c.PhoneNumberDatabaseHMACKeyMaxAge, "PHONE_NUMBER_DATABASE_HMAC_MAX_AGE", 0},
		{c.VerificationCodeDatabaseHMACKeyMinAge, "VERIFICATION_CODE_DATABASE_HMAC_KEY_MIN_AGE", 0},
		{c.VerificationCodeDatabaseHMACKeyMaxAge, "VERIFICATION_CODE_DATABASE_HMAC_KEY_MAX_AGE", 0},
		{c.VerificationSigningKeyMaxAge, "VERIFICATION_SIGNING_KEY_MAX_AGE", 0},
		{c.VerificationActivationDelay, "VERIFICATION_ACTIVATION_DELAY", 0},
		{c.TokenSigningKeyMaxAge, "TOKEN_SIGNING_KEY_MAX_AGE", 0},
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
