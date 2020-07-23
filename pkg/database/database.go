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

// Package database manages database connections and ORM integration.
package database

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/jinzhu/gorm"

	// ensure the postgres dialiect is compiled in.
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Database is a handle to the database layer for the Exposure Notifications
// Verification Server.
type Database struct {
	db     *gorm.DB
	config *Config

	// cacher is an internal write-through cache for frequent lookups.
	cacher *cache.Cache

	// secretManager is used to resolve secrets.
	secretManager secrets.SecretManager
}

// Open created a DB connection through gorm.
func (c *Config) Open(ctx context.Context) (*Database, error) {
	// Create the cacher.
	cacher, err := cache.New(c.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create the secret manager.
	secretManager, err := secrets.SecretManagerFor(ctx, c.Secrets.SecretManagerType)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Connect to the database.
	db, err := gorm.Open("postgres", c.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("database gorm.Open: %w", err)
	}
	return &Database{
		db:            db,
		config:        c,
		cacher:        cacher,
		secretManager: secretManager,
	}, nil
}

// Close will close the database connection. Should be deferred right after Open.
func (db *Database) Close() error {
	return db.db.Close()
}
