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
	rawDB = rawDB.Set("gorm:auto_preload", true)

	// SMS configs
	rawDB.Callback().Create().Before("gorm:create").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	rawDB.Callback().Create().After("gorm:create").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	rawDB.Callback().Update().Before("gorm:update").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	rawDB.Callback().Update().After("gorm:update").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	rawDB.Callback().Query().After("gorm:after_query").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	// Verification codes
	rawDB.Callback().Create().Before("gorm:create").Register("verification_codes:hmac_code", callbackHMAC(ctx, db.hmacVerificationCode, "verification_codes", "code"))
	rawDB.Callback().Create().Before("gorm:create").Register("verification_codes:hmac_long_code", callbackHMAC(ctx, db.hmacVerificationCode, "verification_codes", "long_code"))

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
			scope.Log("skipping decryption, model has errors")
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
			scope.Log("skipping encryption, model has errors")
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
