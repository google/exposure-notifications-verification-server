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
	"errors"
	"fmt"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/jinzhu/gorm"
	"go.uber.org/zap"

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

	// keyManager is used to encrypt/decrypt values.
	keyManager keys.KeyManager

	// logger is the internal logger.
	logger *zap.SugaredLogger

	// secretManager is used to resolve secrets.
	secretManager secrets.SecretManager
}

// Load loads the configuration and processes any dependencies like secret and
// key managers. It does NOT connect to the database.
func (c *Config) Load(ctx context.Context) (*Database, error) {
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

	// Create the key manager.
	keyManager, err := keys.KeyManagerFor(ctx, c.Keys.KeyManagerType)
	if err != nil {
		return nil, fmt.Errorf("failed to create key manager: %w", err)
	}

	// If the key manager is in-memory, accept the key as a base64-encoded
	// in-memory key.
	if c.Keys.KeyManagerType == keys.KeyManagerTypeInMemory {
		typ, ok := keyManager.(keys.EncryptionKeyAdder)
		if !ok {
			return nil, fmt.Errorf("key manager does not support adding keys")
		}

		key, err := base64util.DecodeString(c.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encryption key is invalid: %w", err)
		}

		if err := typ.AddEncryptionKey("database-encryption-key", key); err != nil {
			return nil, fmt.Errorf("failed to add encryption key: %w", err)
		}
		c.EncryptionKey = "database-encryption-key"
	}

	logger := logging.FromContext(ctx).Named("database")

	return &Database{
		config:        c,
		cacher:        cacher,
		keyManager:    keyManager,
		logger:        logger,
		secretManager: secretManager,
	}, nil
}

// Open creates a database connection. This should only be called once.
func (db *Database) Open(ctx context.Context) error {
	c := db.config

	rawDB, err := gorm.Open("postgres", c.ConnectionString())
	if err != nil {
		return fmt.Errorf("database gorm.Open: %w", err)
	}

	// Log SQL statements in debug mode.
	if c.Debug {
		rawDB = rawDB.LogMode(true)
	}

	// Enable auto-preloading.
	rawDB.Set("gorm:auto_preload", true)

	db.db = rawDB
	return nil
}

// Close will close the database connection. Should be deferred right after Open.
func (db *Database) Close() error {
	return db.db.Close()
}

// Ping attempts a connection and closes it to the database.
func (db *Database) Ping(ctx context.Context) error {
	return db.db.DB().PingContext(ctx)
}

// IsNotFound determines if an error is a record not found.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) || gorm.IsRecordNotFoundError(err)
}
