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

package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/sethvargo/go-envconfig"
)

// Config represents the env var based configuration for database connections.
type Config struct {
	Name              string `env:"DB_NAME" json:",omitempty"`
	User              string `env:"DB_USER" json:",omitempty"`
	Host              string `env:"DB_HOST, default=localhost" json:",omitempty"`
	Port              string `env:"DB_PORT, default=5432" json:",omitempty"`
	SSLMode           string `env:"DB_SSLMODE, default=require" json:",omitempty"`
	ConnectionTimeout uint   `env:"DB_CONNECT_TIMEOUT" json:",omitempty"`
	Password          string `env:"DB_PASSWORD" json:"-"` // ignored by zap's JSON formatter
	SSLCertPath       string `env:"DB_SSLCERT" json:",omitempty"`
	SSLKeyPath        string `env:"DB_SSLKEY" json:",omitempty"`
	SSLRootCertPath   string `env:"DB_SSLROOTCERT" json:",omitempty"`

	// MaxConnectionLifetime and MaxConnectionIdleTime determine the connection
	// configuration. Note that MaxConnectionIdleTime must be less than
	// MaxConnectionLifetime.
	MaxConnectionLifetime time.Duration `env:"DB_MAX_CONN_LIFETIME, default=5m" json:",omitempty"`
	MaxConnectionIdleTime time.Duration `env:"DB_MAX_CONN_IDLE_TIME, default=1m" json:",omitempty"`

	// Debug is a boolean that indicates whether the database should log SQL
	// commands.
	Debug bool `env:"DB_DEBUG,default=false"`

	// Keys is the key management configuration. This is used to resolve values
	// that are encrypted via a KMS.
	Keys keys.Config `env:",prefix=DB_"`

	// KeyRing is the KMS keyring to use for managing KMS keys.
	KeyRing string `env:"DB_KEYRING"`

	// MaxKeyVersions is the maximum number of signing key versions for a type,
	// per realm. This is enforced at the database layer, not the upstream KMS.
	MaxKeyVersions int64 `env:"DB_MAX_KEY_VERSIONS, default=5"`

	// EncryptionKey is the reference to an encryption/decryption key to use when
	// for application-layer encryption before values are persisted to the
	// database.
	EncryptionKey string `env:"DB_ENCRYPTION_KEY,required" json:"-"`

	// APIKeyDatabaseHMAC is the HMAC key to use for API keys before storing them
	// in the database.
	APIKeyDatabaseHMAC []envconfig.Base64Bytes `env:"DB_APIKEY_DATABASE_KEY,required" json:"-"`

	// APIKeySignatureHMAC is the HMAC key to sign API keys before returning them
	// to the requestor.
	APIKeySignatureHMAC []envconfig.Base64Bytes `env:"DB_APIKEY_SIGNATURE_KEY,required" json:"-"`

	// VerificationCodeDatabaseHMAC is the HMAC key to hash codes before storing
	// them in the database.
	VerificationCodeDatabaseHMAC []envconfig.Base64Bytes `env:"DB_VERIFICATION_CODE_DATABASE_KEY,required" json:"-"`

	// Secrets is the secret configuration. This is used to resolve values that
	// are actually pointers to secrets before returning them to the caller. The
	// table implementation is the source of truth for which values are secrets
	// and which are plaintext.
	Secrets secrets.Config
}

// ConnectionString returns the postgresql connection string based on this config.
//
// While this package could be adapted to different databases easily, this file
// and method in particular would need to change.
func (c *Config) ConnectionString() string {
	vals := dbValues(c)

	p := make([]string, 0, len(vals))
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(p, " ")
}

// clone creates a deep copy of the configuration.
func (c *Config) clone() *Config {
	cfg := &Config{
		Name:                  c.Name,
		User:                  c.User,
		Host:                  c.Host,
		Port:                  c.Port,
		SSLMode:               c.SSLMode,
		ConnectionTimeout:     c.ConnectionTimeout,
		Password:              c.Password,
		SSLCertPath:           c.SSLCertPath,
		SSLKeyPath:            c.SSLKeyPath,
		SSLRootCertPath:       c.SSLRootCertPath,
		MaxConnectionLifetime: c.MaxConnectionLifetime,
		MaxConnectionIdleTime: c.MaxConnectionIdleTime,
		Debug:                 c.Debug,
		Keys: keys.Config{
			KeyManagerType: c.Keys.KeyManagerType,
			CreateHSMKeys:  c.Keys.CreateHSMKeys,
			FilesystemRoot: c.Keys.FilesystemRoot,
		},
		KeyRing:        c.KeyRing,
		MaxKeyVersions: c.MaxKeyVersions,
		EncryptionKey:  c.EncryptionKey,
		Secrets: secrets.Config{
			SecretManagerType: c.Secrets.SecretManagerType,
			SecretsDir:        c.Secrets.SecretsDir,
			SecretCacheTTL:    c.Secrets.SecretCacheTTL,
			SecretExpansion:   c.Secrets.SecretExpansion,
		},
	}

	cfg.APIKeyDatabaseHMAC = make([]envconfig.Base64Bytes, len(c.APIKeyDatabaseHMAC))
	copy(cfg.APIKeyDatabaseHMAC, c.APIKeyDatabaseHMAC)

	cfg.APIKeySignatureHMAC = make([]envconfig.Base64Bytes, len(c.APIKeySignatureHMAC))
	copy(cfg.APIKeySignatureHMAC, c.APIKeySignatureHMAC)

	cfg.VerificationCodeDatabaseHMAC = make([]envconfig.Base64Bytes, len(c.VerificationCodeDatabaseHMAC))
	copy(cfg.VerificationCodeDatabaseHMAC, c.VerificationCodeDatabaseHMAC)

	return cfg
}

func dbValues(config *Config) map[string]string {
	p := map[string]string{}
	setIfNotEmpty(p, "dbname", config.Name)
	setIfNotEmpty(p, "user", config.User)
	setIfNotEmpty(p, "host", config.Host)
	setIfNotEmpty(p, "port", config.Port)
	setIfNotEmpty(p, "sslmode", config.SSLMode)
	setIfPositive(p, "connect_timeout", config.ConnectionTimeout)
	setIfNotEmpty(p, "password", config.Password)
	setIfNotEmpty(p, "sslcert", config.SSLCertPath)
	setIfNotEmpty(p, "sslkey", config.SSLKeyPath)
	setIfNotEmpty(p, "sslrootcert", config.SSLRootCertPath)
	return p
}

func setIfNotEmpty(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}

func setIfPositive(m map[string]string, key string, val uint) {
	if val > 0 {
		m[key] = fmt.Sprintf("%d", val)
	}
}
