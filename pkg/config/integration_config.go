package config

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig"
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

func NewIntegrationTestConfig(ctx context.Context, tb testing.TB) (*IntegrationTestConfig, *database.Database, error) {
	db, dbConfig := database.NewTestDatabaseWithConfig(tb)
	obConfig := &observability.Config{
		ExporterType: "NOOP",
	}

	var cfg IntegrationTestConfig
	if err := ProcessWith(ctx, &cfg, envconfig.OsLookuper()); err != nil {
		return nil, nil, err
	}
	cfg.Observability = obConfig
	cfg.DBConfig = dbConfig

	cfg.APISrvConfig.Database = *dbConfig
	cfg.APISrvConfig.VerificationTokenDuration = VerificationTokenDuration
	cfg.APISrvConfig.APIKeyCacheDuration = APIKeyCacheDuration
	cfg.APISrvConfig.Port = APISrvPort

	cfg.AdminAPISrvConfig.Database = *dbConfig
	cfg.AdminAPISrvConfig.Port = AdminSrvPort

	return &cfg, db, nil
}
