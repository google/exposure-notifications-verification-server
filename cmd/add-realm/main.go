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

// Adds a new realm.
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

var (
	nameFlag = flag.String("name", "", "name of the realm to add")
)

func main() {
	flag.Parse()

	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLogger(true)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	if *nameFlag == "" {
		return fmt.Errorf("--name must be passed and cannot be empty")
	}

	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := config.Load(ctx)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	if err := db.Open(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	realm := database.Realm{
		Name: *nameFlag,
	}
	if err := db.SaveRealm(&realm); err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}

	logger.Infow("created realm", "realm", realm)
	return nil
}
