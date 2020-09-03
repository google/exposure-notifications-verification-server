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
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/jinzhu/gorm"
	"github.com/sethvargo/go-retry"
	"go.uber.org/zap"

	// ensure the postgres dialiect is compiled in.
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// Database is a handle to the database layer for the Exposure Notifications
// Verification Server.
type Database struct {
	db     *gorm.DB
	config *Config

	// keyManager is used to encrypt/decrypt values.
	keyManager keys.KeyManager

	// signingKeyManager is an optional interface that's implemented to support
	// per-realm signing keys. This could be nil.
	signingKeyManager keys.SigningKeyManager

	// logger is the internal logger.
	logger *zap.SugaredLogger

	// secretManager is used to resolve secrets.
	secretManager secrets.SecretManager
}

// SupportsPerRealmSigning returns true if the configuration supports
// application managed signing keys.
func (db *Database) SupportsPerRealmSigning() bool {
	return db.signingKeyManager != nil
}

func (db *Database) KeyManager() keys.KeyManager {
	return db.keyManager
}

// Load loads the configuration and processes any dependencies like secret and
// key managers. It does NOT connect to the database.
func (c *Config) Load(ctx context.Context) (*Database, error) {
	logger := logging.FromContext(ctx).Named("database")

	// Create the secret manager.
	secretManager, err := secrets.SecretManagerFor(ctx, c.Secrets.SecretManagerType)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Create the key manager.
	keyManager, err := keys.KeyManagerFor(ctx, &c.Keys)
	if err != nil {
		return nil, fmt.Errorf("failed to create key manager: %w", err)
	}

	var signingKeyManager keys.SigningKeyManager
	signingKeyManager, ok := keyManager.(keys.SigningKeyManager)
	if !ok {
		signingKeyManager = nil
		logger.Errorf("key manager does not support the SigningKeyManager interface, falling back to single verification signing key")
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

	return &Database{
		config:            c,
		keyManager:        keyManager,
		signingKeyManager: signingKeyManager,
		logger:            logger,
		secretManager:     secretManager,
	}, nil
}

// Open creates a database connection. This should only be called once.
func (db *Database) Open(ctx context.Context) error {
	return db.OpenWithCacher(ctx, nil)
}

// OpenWithCacher creates a database connection with the cacher. This should
// only be called once.
func (db *Database) OpenWithCacher(ctx context.Context, cacher cache.Cacher) error {
	c := db.config

	// Establish a connection to the database.
	b, err := retry.NewFibonacci(250 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to configure database backoff: %w", err)
	}
	b = retry.WithMaxRetries(10, b)
	b = retry.WithCappedDuration(2*time.Second, b)

	var rawDB *gorm.DB
	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		var err error
		rawDB, err = gorm.Open("postgres", c.ConnectionString())
		if err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	if rawDB == nil {
		return fmt.Errorf("failed to create database connection")
	}

	// Set connection configuration.
	rawDB.DB().SetConnMaxLifetime(c.MaxConnectionLifetime)
	rawDB.DB().SetConnMaxIdleTime(c.MaxConnectionIdleTime)

	// Log SQL statements in debug mode.
	if c.Debug {
		rawDB = rawDB.LogMode(true)
	}

	// Enable auto-preloading.
	rawDB = rawDB.Set("gorm:auto_preload", true)

	callbacks := rawDB.Callback()

	// SMS configs
	callbacks.Create().Before("gorm:create").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	callbacks.Create().After("gorm:create").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	callbacks.Update().Before("gorm:update").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	callbacks.Update().After("gorm:update").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	callbacks.Query().After("gorm:after_query").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	// Verification codes
	callbacks.Create().Before("gorm:create").Register("verification_codes:hmac_code", callbackHMAC(ctx, db.GenerateVerificationCodeHMAC, "verification_codes", "code"))
	callbacks.Create().Before("gorm:create").Register("verification_codes:hmac_long_code", callbackHMAC(ctx, db.GenerateVerificationCodeHMAC, "verification_codes", "long_code"))

	// Cache clearing
	if cacher != nil {
		// Apps
		callbacks.Update().After("gorm:update").Register("purge_cache:authorized_apps:by_id", callbackPurgeCache(ctx, cacher, "authorized_apps:by_id:%d", "authorized_apps", "id"))
		callbacks.Delete().After("gorm:delete").Register("purge_cache:authorized_apps:by_id", callbackPurgeCache(ctx, cacher, "authorized_apps:by_id:%d", "authorized_apps", "id"))

		// Realms
		callbacks.Update().After("gorm:update").Register("purge_cache:realms:by_id", callbackPurgeCache(ctx, cacher, "realms:by_id:%d", "realms", "id"))
		callbacks.Delete().After("gorm:delete").Register("purge_cache:realms:by_id", callbackPurgeCache(ctx, cacher, "realms:by_id:%d", "realms", "id"))

		// Users
		callbacks.Update().After("gorm:update").Register("purge_cache:users:by_id", callbackPurgeCache(ctx, cacher, "users:by_id:%d", "users", "id"))
		callbacks.Delete().After("gorm:delete").Register("purge_cache:users:by_id", callbackPurgeCache(ctx, cacher, "users:by_id:%d", "users", "id"))

		// Users (by email)
		callbacks.Update().After("gorm:update").Register("purge_cache:users:by_email", callbackPurgeCache(ctx, cacher, "users:by_email:%s", "users", "email"))
		callbacks.Delete().After("gorm:delete").Register("purge_cache:users:by_email", callbackPurgeCache(ctx, cacher, "users:by_email:%s", "users", "email"))

	}

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

// callbackPurgeCache purges the cache key for the given record.
func callbackPurgeCache(ctx context.Context, cacher cache.Cacher, keyFormat, table, column string) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		if scope.TableName() != table {
			return
		}

		if scope.HasError() {
			return
		}

		field, ok := scope.FieldByName(column)
		if !ok {
			_ = scope.Err(fmt.Errorf("table %q has no column %q", table, column))
			return
		}

		if !field.Field.CanInterface() {
			_ = scope.Err(fmt.Errorf("%q.%q cannot interface", table, column))
			return
		}

		val := field.Field.Interface()
		if val == nil {
			return
		}

		key := fmt.Sprintf(keyFormat, val)
		if err := cacher.Delete(ctx, key); err != nil {
			scope.Log(fmt.Sprintf("failed to delete cache key: %v", err))
			return
		}

		scope.Log(fmt.Sprintf("cleared cache for %v", key))
	}
}

// callbackKMSDecrypt decrypts the given column in the table using the key
// manager and key id.
func callbackKMSDecrypt(ctx context.Context, keyManager keys.KeyManager, keyID, table, column string) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		// Do nothing if not the target table
		if scope.TableName() != table {
			return
		}

		// Do nothing if there are errors
		if scope.HasError() {
			return
		}

		realField, ciphertext, hasRealField := getFieldString(scope, column)
		if !hasRealField {
			scope.Log(fmt.Sprintf("skipping decryption, %s is not a string", realField.Name))
			return
		}
		if ciphertext == "" {
			scope.Log(fmt.Sprintf("skipping decryption, %s is blank", realField.Name))
			return
		}

		plaintextCacheField, plaintextCache, hasPlaintextCache := getFieldString(scope, column+"PlaintextCache")
		ciphertextCacheField, ciphertextCache, hasCiphertextCache := getFieldString(scope, column+"CiphertextCache")

		// Optimization - if PlaintextCache and CiphertextCache columns exist and the
		// ciphertext is unchanged, do not decrypt.
		if hasPlaintextCache && hasCiphertextCache && ciphertext == ciphertextCache {
			if err := realField.Set(plaintextCache); err != nil {
				_ = scope.Err(fmt.Errorf("failed to re-use plaintext: %w", err))
				return
			}
		}

		ciphertextBytes, err := base64util.DecodeString(ciphertext)
		if err != nil {
			_ = scope.Err(fmt.Errorf("cannot decrypt %s, invalid ciphertext", realField.Name))
			return
		}

		plaintextBytes, err := keyManager.Decrypt(ctx, keyID, ciphertextBytes, nil)
		if err != nil {
			_ = scope.Err(fmt.Errorf("failed to decrypt %s: %w", column, err))
			return
		}
		plaintext := string(plaintextBytes)

		if hasRealField {
			if err := realField.Set(plaintext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", realField.Name, err))
				return
			}
		}

		if hasPlaintextCache {
			if err := plaintextCacheField.Set(plaintext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", plaintextCacheField.Name, err))
				return
			}
		}

		if hasCiphertextCache {
			if err := ciphertextCacheField.Set(ciphertext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", ciphertextCacheField.Name, err))
				return
			}
		}
	}
}

// callbackKMSEncrypt encrypts the given column in the table using the key
// manager and key id before saving in the database.
func callbackKMSEncrypt(ctx context.Context, keyManager keys.KeyManager, keyID, table, column string) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		// Do nothing if not the target table
		if scope.TableName() != table {
			return
		}

		// Do nothing if there are errors
		if scope.HasError() {
			return
		}

		realField, plaintext, hasRealField := getFieldString(scope, column)
		if !hasRealField {
			scope.Log(fmt.Sprintf("skipping encryption, %s is not a string", realField.Name))
			return
		}
		if plaintext == "" {
			scope.Log(fmt.Sprintf("skipping encryption, %s is blank", realField.Name))
			return
		}

		plaintextCacheField, plaintextCache, hasPlaintextCache := getFieldString(scope, column+"PlaintextCache")
		ciphertextCacheField, ciphertextCache, hasCiphertextCache := getFieldString(scope, column+"CiphertextCache")

		// Optimization - if PlaintextCache and CiphertextCache columns exist and the
		// plaintext is unchanged, do not re-encrypt.
		if hasPlaintextCache && hasCiphertextCache && plaintext == plaintextCache {
			if err := realField.Set(ciphertextCache); err != nil {
				_ = scope.Err(fmt.Errorf("failed to re-use encrypted ciphertext: %w", err))
				return
			}
		}

		b, err := keyManager.Encrypt(ctx, keyID, []byte(plaintext), nil)
		if err != nil {
			_ = scope.Err(fmt.Errorf("failed to encrypt %s: %w", column, err))
			return
		}
		ciphertext := base64.RawStdEncoding.EncodeToString(b)

		if hasRealField {
			if err := realField.Set(ciphertext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", realField.Name, err))
				return
			}
		}

		if hasPlaintextCache {
			if err := plaintextCacheField.Set(plaintext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", plaintextCacheField.Name, err))
				return
			}
		}

		if hasCiphertextCache {
			if err := ciphertextCacheField.Set(ciphertext); err != nil {
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", ciphertextCacheField.Name, err))
				return
			}
		}
	}
}

// callback HMAC alters HMACs the value with the given key before saving.
func callbackHMAC(ctx context.Context, hashFunc func(string) (string, error), table, column string) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		// Do nothing if not the target table
		if scope.TableName() != table {
			return
		}

		// Do nothing if there are errors
		if scope.HasError() {
			return
		}

		field, value, ok := getFieldString(scope, column)
		if !ok {
			scope.Log(fmt.Sprintf("skipping HMAC, %s is not a string", field.Name))
			return
		}
		if value == "" {
			scope.Log(fmt.Sprintf("skipping HMAC, %s is blank", field.Name))
			return
		}

		sig, err := hashFunc(value)
		if err != nil {
			_ = scope.Err(fmt.Errorf("failed to generate HMAC for column %s: %w", field.Name, err))
			return
		}

		if err := field.Set(sig); err != nil {
			_ = scope.Err(fmt.Errorf("failed to set column %s: %w", field.Name, err))
			return
		}
	}
}

func getFieldString(scope *gorm.Scope, name string) (*gorm.Field, string, bool) {
	field, ok := scope.FieldByName(name)
	if !ok {
		return field, "", false
	}

	if !field.Field.CanInterface() {
		return field, "", false
	}

	val := field.Field.Interface()
	if val == nil {
		return field, "", false
	}

	typ, ok := val.(string)
	if !ok {
		return field, "", false
	}

	return field, typ, true
}
