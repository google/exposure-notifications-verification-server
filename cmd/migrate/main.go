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

// A binary for running database migrations
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	target := flag.String("id", "", "migration ID to move to")
	rollback := flag.Bool("rollback", false, "if true, will run a rollback migration towards the target")
	flag.Parse()
	ctx, done := signalcontext.OnInterrupt()

	err := realMain(ctx, *target, *rollback)
	done()

	logger := logging.FromContext(ctx)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("migrations complete")
}

func realMain(ctx context.Context, target string, rollback bool) error {
	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	db, err := config.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.MigrateTo(ctx, target, rollback); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
