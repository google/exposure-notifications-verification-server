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
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/logging"
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/jinzhu/gorm"
	"github.com/sethvargo/go-retry"
	"go.opencensus.io/stats"
	"go.uber.org/zap"

	// ensure the postgres dialiect is compiled in.
	"contrib.go.opencensus.io/integrations/ocsql"
	postgres "github.com/lib/pq"
)

var (
	// callbackLock prevents multiple callbacks from being registered
	// simultaneously because that's a data race in gorm.
	callbackLock sync.Mutex
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

	statsCloser func()
}

// Overrides the postgresql driver with
func init() {
	for _, v := range sql.Drivers() {
		if v == enobservability.OCSQLDriverName {
			return
		}
	}
	sql.Register(enobservability.OCSQLDriverName, ocsql.Wrap(&postgres.Driver{}))
}

// SupportsPerRealmSigning returns true if the configuration supports
// application managed signing keys.
func (db *Database) SupportsPerRealmSigning() bool {
	return db.signingKeyManager != nil
}

// MaxCertificateSigningKeyVersions returns the configured maximum.
func (db *Database) MaxCertificateSigningKeyVersions() int64 {
	return db.config.MaxCertificateSigningKeyVersions
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

	// Establish a connection to the database. We use this later to register
	// opencenusus stats.
	var rawSQL *sql.DB
	if err := withRetries(ctx, func(ctx context.Context) error {
		var err error
		rawSQL, err = sql.Open("ocsql", c.ConnectionString())
		if err != nil {
			return retry.RetryableError(err)
		}
		db.statsCloser = ocsql.RecordStats(rawSQL, 5*time.Second)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to create sql connection: %w", err)
	}
	if rawSQL == nil {
		return fmt.Errorf("failed to create database connection")
	}

	var rawDB *gorm.DB
	if err := withRetries(ctx, func(ctx context.Context) error {
		var err error
		// Need to give postgres dialect as otherwise gorm starts running
		// in compatibility mode
		rawDB, err = gorm.Open("postgres", rawSQL)
		if err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to initialize gorm: %w", err)
	}
	if rawDB == nil {
		return fmt.Errorf("failed to initialize gorm")
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

	// Prevent multiple simultaneous callback registrations due to a data race in
	// gorm.
	callbackLock.Lock()
	defer callbackLock.Unlock()

	// Disable the gorm logger here unless were in debug mode. The logs for
	// callbacks are really verbose and unnecessary.
	if !c.Debug {
		rawDB.SetLogger(gorm.Logger{LogWriter: log.New(ioutil.Discard, "", 0)})
		defer rawDB.SetLogger(gorm.Logger{LogWriter: log.New(os.Stdout, "\r\n", 0)})
	}

	// SMS configs
	rawDB.Callback().Create().Before("gorm:create").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	rawDB.Callback().Create().After("gorm:create").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	rawDB.Callback().Update().Before("gorm:update").Register("sms_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))
	rawDB.Callback().Update().After("gorm:update").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	rawDB.Callback().Query().After("gorm:after_query").Register("sms_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "sms_configs", "TwilioAuthToken"))

	// Email configs
	rawDB.Callback().Create().Before("gorm:create").Register("email_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "email_configs", "SMTPPassword"))
	rawDB.Callback().Create().After("gorm:create").Register("email_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "email_configs", "SMTPPassword"))

	rawDB.Callback().Update().Before("gorm:update").Register("email_configs:encrypt", callbackKMSEncrypt(ctx, db.keyManager, c.EncryptionKey, "email_configs", "SMTPPassword"))
	rawDB.Callback().Update().After("gorm:update").Register("email_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "email_configs", "SMTPPassword"))

	rawDB.Callback().Query().After("gorm:after_query").Register("email_configs:decrypt", callbackKMSDecrypt(ctx, db.keyManager, c.EncryptionKey, "email_configs", "SMTPPassword"))

	// Verification codes
	rawDB.Callback().Create().Before("gorm:create").Register("verification_codes:hmac_code", callbackHMAC(ctx, db.GenerateVerificationCodeHMAC, "verification_codes", "code"))
	rawDB.Callback().Create().Before("gorm:create").Register("verification_codes:hmac_long_code", callbackHMAC(ctx, db.GenerateVerificationCodeHMAC, "verification_codes", "long_code"))

	// Metrics
	rawDB.Callback().Create().After("gorm:create").Register("audit_entries:metrics", callbackIncrementMetric(ctx, mAuditEntryCreated, "audit_entries"))

	// Cache clearing
	if cacher != nil {
		// Apps
		rawDB.Callback().Update().After("gorm:update").Register("purge_cache:authorized_apps:by_id", callbackPurgeCache(ctx, cacher, "authorized_apps:by_id", "authorized_apps", "id"))
		rawDB.Callback().Delete().After("gorm:delete").Register("purge_cache:authorized_apps:by_id", callbackPurgeCache(ctx, cacher, "authorized_apps:by_id", "authorized_apps", "id"))

		// Realms
		rawDB.Callback().Update().After("gorm:update").Register("purge_cache:realms:by_id", callbackPurgeCache(ctx, cacher, "realms:by_id", "realms", "id"))
		rawDB.Callback().Delete().After("gorm:delete").Register("purge_cache:realms:by_id", callbackPurgeCache(ctx, cacher, "realms:by_id", "realms", "id"))

		// Users
		rawDB.Callback().Update().After("gorm:update").Register("purge_cache:users:by_id", callbackPurgeCache(ctx, cacher, "users:by_id", "users", "id"))
		rawDB.Callback().Delete().After("gorm:delete").Register("purge_cache:users:by_id", callbackPurgeCache(ctx, cacher, "users:by_id", "users", "id"))

		// Users (by email)
		rawDB.Callback().Update().After("gorm:update").Register("purge_cache:users:by_email", callbackPurgeCache(ctx, cacher, "users:by_email", "users", "email"))
		rawDB.Callback().Delete().After("gorm:delete").Register("purge_cache:users:by_email", callbackPurgeCache(ctx, cacher, "users:by_email", "users", "email"))
	}

	db.db = rawDB
	return nil
}

// Close will close the database connection. Should be deferred right after Open.
func (db *Database) Close() error {
	db.statsCloser()
	return db.db.Close()
}

// Ping attempts a connection and closes it to the database.
func (db *Database) Ping(ctx context.Context) error {
	return db.db.DB().PingContext(ctx)
}

// RawDB returns the underlying gorm database.
func (db *Database) RawDB() *gorm.DB {
	return db.db
}

// IsNotFound determines if an error is a record not found.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound) || gorm.IsRecordNotFoundError(err)
}

// callbackIncrementMetric increments the provided metric
func callbackIncrementMetric(ctx context.Context, m *stats.Int64Measure, table string) func(scope *gorm.Scope) {
	return func(scope *gorm.Scope) {
		if scope.TableName() != table {
			return
		}

		if scope.HasError() {
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		field, ok := scope.FieldByName("realm_id")
		if ok && field.Field.CanInterface() && field.Field.Interface() != nil {
			realmIDRaw := field.Field.Interface()

			var realmID uint
			switch t := realmIDRaw.(type) {
			case uint:
				realmID = t
			case uint8:
				realmID = uint(t)
			case uint16:
				realmID = uint(t)
			case uint32:
				realmID = uint(t)
			case uint64:
				realmID = uint(t)
			case int:
				realmID = uint(t)
			case int8:
				realmID = uint(t)
			case int16:
				realmID = uint(t)
			case int32:
				realmID = uint(t)
			case int64:
				realmID = uint(t)
			case string:
				raw, err := strconv.ParseUint(t, 10, 64)
				if err != nil {
					_ = scope.Err(fmt.Errorf("failed to parse realm_id: %w", err))
					return
				}

				if raw >= 1<<bits.UintSize-1 {
					_ = scope.Err(fmt.Errorf("uint overflows %d bits", bits.UintSize))
					return
				}
				realmID = uint(raw)
			default:
				_ = scope.Err(fmt.Errorf("realm_id is of unknown type %v", t))
				return
			}

			ctx = observability.WithRealmID(ctx, realmID)
		}

		ctx = observability.WithBuildInfo(ctx)
		stats.Record(ctx, m.M(1))
	}
}

// callbackPurgeCache purges the cache key for the given record.
func callbackPurgeCache(ctx context.Context, cacher cache.Cacher, namespace, table, column string) func(scope *gorm.Scope) {
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

		key := &cache.Key{
			Namespace: namespace,
			Key:       fmt.Sprintf("%v", val),
		}
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
			scope.Log(fmt.Sprintf("skipping decryption, %s is not a string", column))
			return
		}
		if ciphertext == "" {
			scope.Log(fmt.Sprintf("skipping decryption, %s is blank", column))
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
			_ = scope.Err(fmt.Errorf("cannot decrypt %s, invalid ciphertext", column))
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
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", column, err))
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
			scope.Log(fmt.Sprintf("skipping encryption, %s is not a string", column))
			return
		}
		if plaintext == "" {
			scope.Log(fmt.Sprintf("skipping encryption, %s is blank", column))
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
				_ = scope.Err(fmt.Errorf("failed to set column %s: %w", column, err))
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

// callbackHMAC alters HMACs the value with the given key before saving.
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

// withRetries is a helper for creating a fibonacci backoff with capped retries,
// useful for retrying database queries.
func withRetries(ctx context.Context, f retry.RetryFunc) error {
	b, err := retry.NewConstant(500 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to configure backoff: %w", err)
	}
	b = retry.WithMaxRetries(30, b)

	return retry.Do(ctx, b, f)
}

// stringValue gets the value of the string pointer, returning "" for nil.
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// stringPtr converts the string value to a pointer, returning nil for "".
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// stringDiff builds a diff of the string values.
func stringDiff(old, new string) string {
	var w strings.Builder

	for _, line := range strings.Split(old, "\n") {
		fmt.Fprintf(&w, "-%s\n", line)
	}

	for _, line := range strings.Split(new, "\n") {
		fmt.Fprintf(&w, "+%s\n", line)
	}

	return w.String()
}

func boolDiff(old, new bool) string {
	return stringDiff(strconv.FormatBool(old), strconv.FormatBool(new))
}

func float32Diff(old, new float32) string {
	return float64Diff(float64(old), float64(new))
}

func float64Diff(old, new float64) string {
	return stringDiff(strconv.FormatFloat(old, 'f', 4, 64), strconv.FormatFloat(new, 'f', 4, 64))
}

func uintDiff(old, new uint) string {
	return stringDiff(strconv.FormatUint(uint64(old), 10), strconv.FormatUint(uint64(new), 10))
}
