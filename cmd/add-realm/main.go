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
	"os"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-signalcontext"
)

var (
	name = flag.String("name", "", "name of the realm to add")
)

func main() {
	ctx, done := signalcontext.OnInterrupt()
	err := realMain(ctx)
	done()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func realMain(ctx context.Context) error {
	flag.Parse()

	if *name == "" {
		return fmt.Errorf("--name must be passed and cannot be empty")
	}

	var config database.Config
	if err := envconfig.Process(ctx, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	db, err := config.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	realm := database.Realm{
		Name: *name,
	}
	if err := db.SaveRealm(&realm); err != nil {
		return fmt.Errorf("failed to create realm: %w", err)
	}

	fmt.Printf("successfully created realm %v (%v)\n", realm.Name, realm.ID)
	return nil
}
