package config

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
)

const (
	VerificationTokenDuration = time.Second * 2
	APIKeyCacheDuration       = time.Second * 2

	APISrvPort   = "8080"
	AdminSrvPort = "8081"
)

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
	tb.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			tb.Fatal(err)
		}
	})

	parent := keys.TestSigningKey(tb, kms)
	keyID, err := kms.(keys.SigningKeyManager).CreateKeyVersion(ctx, parent)
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
