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
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
	"github.com/sethvargo/go-envconfig"
)

const (
	VerificationTokenDuration = time.Second * 2
	APIKeyCacheDuration       = time.Second * 2

	APISrvPort   = "8080"
	AdminSrvPort = "8081"
)

// IntegrationTestConfig represents configurations to run server integration tests.
type IntegrationTestConfig struct {
	Observability *observability.Config
	DBConfig      *database.Config

	APISrvConfig      APIServerConfig
	AdminAPISrvConfig AdminAPIServerConfig
}

func NewIntegrationTestConfig(ctx context.Context, tb testing.TB) (*IntegrationTestConfig, *database.Database) {
	db, dbConfig := database.NewTestDatabaseWithConfig(tb)
	obConfig := &observability.Config{
		ExporterType: "NOOP",
	}

	cacheConfig := cache.Config{
		Type: "IN_MEMORY",
	}

	rlConfig := ratelimit.Config{
		Type: "NOOP",
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		tb.Fatal(err)
	}
	keyConfig := keys.Config{
		KeyManagerType: "FILESYSTEM",
		FilesystemRoot: tmpdir,
	}

	kms, err := keys.KeyManagerFor(ctx, &keyConfig)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			tb.Fatal(err)
		}
	})

	parent := keys.TestSigningKey(tb, kms)
	skm, ok := kms.(keys.SigningKeyManager)
	if !ok {
		tb.Fatal("KMS doesn't implement interface SigningKeyManager")
	}
	keyID, err := skm.CreateKeyVersion(ctx, parent)
	if err != nil {
		tb.Fatal(err)
	}

	tsConfig := TokenSigningConfig{
		Keys:               keyConfig,
		TokenSigningKeys:   []string{keyID},
		TokenSigningKeyIDs: []string{"v1"},
		TokenIssuer:        "diagnosis-verification-example",
	}

	csConfig := CertificateSigningConfig{
		Keys:                    keyConfig,
		PublicKeyCacheDuration:  15 * time.Minute,
		SignerCacheDuration:     time.Minute,
		CertificateSigningKey:   keyID,
		CertificateSigningKeyID: "v1",
		CertificateIssuer:       "diagnosis-verification-example",
		CertificateAudience:     "exposure-notifications-server",
		CertificateDuration:     15 * time.Minute,
	}

	cfg := IntegrationTestConfig{
		Observability: obConfig,
		DBConfig:      dbConfig,
		APISrvConfig: APIServerConfig{
			Database:                  *dbConfig,
			Observability:             *obConfig,
			Cache:                     cacheConfig,
			DevMode:                   true,
			Port:                      APISrvPort,
			APIKeyCacheDuration:       APIKeyCacheDuration,
			VerificationTokenDuration: VerificationTokenDuration,
			TokenSigning:              tsConfig,
			CertificateSigning:        csConfig,
			RateLimit:                 rlConfig,
		},
		AdminAPISrvConfig: AdminAPIServerConfig{
			Database:            *dbConfig,
			Observability:       *obConfig,
			Cache:               cacheConfig,
			DevMode:             true,
			RateLimit:           rlConfig,
			Port:                AdminSrvPort,
			APIKeyCacheDuration: APIKeyCacheDuration,
			CollisionRetryCount: 6,
			AllowedSymptomAge:   time.Hour * 336,
		},
	}

	return &cfg, db
}

// E2EConfig represents configurations to run server E2E tests.
type E2EConfig struct {
	APIServerURL string           `env:"E2E_APISERVER_URL"`
	AdminAPIURL  string           `env:"E2E_ADMINAPI_URL"`
	ProjectID    string           `env:"PROJECT_ID"`
	DBConfig     *database.Config `env:",prefix:E2E_"`
}

// NewE2EConfig returns a new E2E test config.
func NewE2EConfig(tb testing.TB, ctx context.Context) *E2EConfig {
	c := &E2EConfig{}
	sm, err := secrets.SecretManagerFor(ctx, secrets.SecretManagerTypeGoogleSecretManager)
	if err != nil {
		tb.Fatalf("unable to connect to secret manager: %v", err)
	}
	if err := envconfig.ProcessWith(ctx, c, envconfig.PrefixLookuper("E2E_", envconfig.OsLookuper()), secrets.Resolver(sm, &secrets.Config{})); err != nil {
		tb.Fatalf("Unable to process environment: %v", err)
	}
	return c
}
