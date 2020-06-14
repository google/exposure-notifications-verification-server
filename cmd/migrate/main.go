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
	"log"

	"github.com/google/exposure-notifications-verification-server/pkg/config"

	_ "github.com/jinzhu/gorm/dialects/postgres"
)

func main() {
	ctx := context.Background()
	config, err := config.New(ctx)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := config.Database.Open()
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}

	err = db.RunMigrations(ctx)
	if err == nil {
		log.Printf("database migrations completed successfully")
	} else {
		log.Fatalf("migration error %v", err)
	}
}
