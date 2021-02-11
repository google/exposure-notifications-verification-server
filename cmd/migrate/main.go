// A binary for running database migrations
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/logging"

	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

var (
	targetFlag   = flag.String("id", "", "migration ID to move to")
	rollbackFlag = flag.Bool("rollback", false, "if true, will run a rollback migration towards the target")
)

func main() {
	flag.Parse()

	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().
		With("build_id", buildinfo.BuildID).
		With("build_tag", buildinfo.BuildTag)
	ctx = logging.WithLogger(ctx, logger)

	defer func() {
		done()
		if r := recover(); r != nil {
			logger.Fatalw("application panic", "panic", r)
		}
	}()

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	var cfg database.Config
	if err := config.ProcessWith(ctx, &cfg, envconfig.OsLookuper()); err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	db, err := cfg.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.MigrateTo(ctx, *targetFlag, *rollbackFlag); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
