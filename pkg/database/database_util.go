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

package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jinzhu/gorm"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/sethvargo/go-envconfig"

	// imported to register the postgres migration driver
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// imported to register the "file" source migration driver
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// imported to register the "postgres" database driver for migrate
)

const (
	// databaseName is the name of the template database to clone.
	databaseName = "test-db-template"

	// databaseUser and databasePassword are the username and password for
	// connecting to the database. These values are only used for testing.
	databaseUser     = "test-user"
	databasePassword = "testing123"

	// defaultPostgresImageRef is the default database container to use if none is
	// specified.
	defaultPostgresImageRef = "postgres:13-alpine"
)

// ApproxTime is a compare helper for clock skew.
var ApproxTime = cmp.Options{cmpopts.EquateApproxTime(1 * time.Second)}

// TestInstance is a wrapper around the Docker-based database instance.
type TestInstance struct {
	pool      *dockertest.Pool
	container *dockertest.Resource

	db     *Database
	dbLock sync.Mutex

	tmpdir     string
	skipReason string
}

// MustTestInstance is NewTestInstance, except it prints errors to stderr and
// calls os.Exit when finished. Callers can call Close or MustClose().
func MustTestInstance() *TestInstance {
	testDatabaseInstance, err := NewTestInstance()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return testDatabaseInstance
}

// NewTestInstance creates a new Docker-based database instance. It also creates
// an initial database, runs the migrations, and sets that database as a
// template to be cloned by future tests.
//
// This should not be used outside of testing, but it is exposed in the package
// so it can be shared with other packages. It should be called and instantiated
// in TestMain.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestInstance() (*TestInstance, error) {
	// Querying for -short requires flags to be parsed.
	if !flag.Parsed() {
		flag.Parse()
	}

	// Do not create an instance in -short mode.
	if testing.Short() {
		return &TestInstance{
			skipReason: "🚧 Skipping database tests (-short flag provided)!",
		}, nil
	}

	// Do not create an instance if database tests are explicitly skipped.
	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_DATABASE_TESTS")); skip {
		return &TestInstance{
			skipReason: "🚧 Skipping database tests (SKIP_DATABASE_TESTS is set)!",
		}, nil
	}

	ctx := context.Background()

	// Create the pool.
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("failed to create database docker pool: %w", err)
	}

	// Determine the container image to use.
	repository, tag, err := postgresRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to determine database repository: %w", err)
	}

	// Start the actual container.
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: repository,
		Tag:        tag,
		Env: []string{
			"LANG=C",
			"POSTGRES_DB=" + databaseName,
			"POSTGRES_USER=" + databaseUser,
			"POSTGRES_PASSWORD=" + databasePassword,
		},
	}, func(c *docker.HostConfig) {
		c.AutoRemove = true
		c.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start database container: %w", err)
	}

	// Stop the container after its been running for too long. No since test suite
	// should take super long.
	if err := container.Expire(120); err != nil {
		return nil, fmt.Errorf("failed to expire database container: %w", err)
	}

	// Generate keys.
	apiKeyDatabaseHMAC, err := generateKeys(2, 128)
	if err != nil {
		return nil, fmt.Errorf("failed to generate api key database hmac: %w", err)
	}
	apiKeySignatureHMAC, err := generateKeys(2, 128)
	if err != nil {
		return nil, fmt.Errorf("failed to generate api key signature hmac: %w", err)
	}
	verificationCodeDatabaseHMAC, err := generateKeys(2, 128)
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification code database hmac: %w", err)
	}

	// Create a temporary directory for the key manager,
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to make tmpdir: %w", err)
	}

	// Build database configuration. This is required to connect to the database
	// and to run the initial migrations.
	config := &Config{
		APIKeyDatabaseHMAC:           apiKeyDatabaseHMAC,
		APIKeySignatureHMAC:          apiKeySignatureHMAC,
		VerificationCodeDatabaseHMAC: verificationCodeDatabaseHMAC,

		Host:     container.GetBoundIP("5432/tcp"),
		Port:     container.GetPort("5432/tcp"),
		User:     databaseUser,
		Name:     databaseName,
		Password: databasePassword,
		SSLMode:  "disable",
		Secrets: secrets.Config{
			Type: "IN_MEMORY",
		},

		Keys: keys.Config{
			Type:           "FILESYSTEM",
			FilesystemRoot: tmpdir,
		},

		KeyRing:        "test-keyring",
		MaxKeyVersions: 5,
	}

	// Parse configuration and override with test data.
	db, err := config.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load database configuration: %w", err)
	}

	// Try to establish a connection to the database.
	if err := db.Open(ctx); err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	db.db.SetLogger(gorm.Logger{LogWriter: log.New(ioutil.Discard, "", 0)})
	db.db.LogMode(false)

	// Run database migrations.
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Return the instance.
	return &TestInstance{
		pool:      pool,
		container: container,
		db:        db,
		tmpdir:    tmpdir,
	}, nil
}

// MustClose is like Close except it prints the error to stderr and calls os.Exit.
func (i *TestInstance) MustClose() error {
	if err := i.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	return nil
}

// Close terminates the test database instance, cleaning up any resources.
func (i *TestInstance) Close() (retErr error) {
	// Do not attempt to close things when there's nothing to close.
	if i.skipReason != "" {
		return
	}

	defer func() {
		if err := os.RemoveAll(i.tmpdir); err != nil {
			retErr = fmt.Errorf("failed to remove tmpdir: %w", err)
			return
		}
	}()

	defer func() {
		if err := i.pool.Purge(i.container); err != nil {
			retErr = fmt.Errorf("failed to purge database container: %w", err)
			return
		}
	}()

	if err := i.db.Close(); err != nil {
		retErr = fmt.Errorf("failed to close connection: %w", err)
		return
	}

	return
}

// UtilOption is used as optional configuration to the database setup.
type UtilOption func(*Database, *Config) (*Database, *Config)

// WithKeyManager alters the key manager.
func WithKeyManager(mcfg *keys.Config, manager keys.KeyManager) UtilOption {
	return func(db *Database, cfg *Config) (*Database, *Config) {
		db.keyManager = manager
		cfg.Keys = *mcfg
		return db, cfg
	}
}

// WithSigningKeyManager alters the signing key manager.
func WithSigningKeyManager(mcfg *keys.Config, manager keys.SigningKeyManager) UtilOption {
	return func(db *Database, cfg *Config) (*Database, *Config) {
		cfg.Keys = *mcfg
		db.signingKeyManager = manager
		return db, cfg
	}
}

// WithSecretManager alters the secret manager.
func WithSecretManager(mcfg *secrets.Config, manager secrets.SecretManager) UtilOption {
	return func(db *Database, cfg *Config) (*Database, *Config) {
		cfg.Secrets = *mcfg
		db.secretManager = manager
		return db, cfg
	}
}

// NewDatabase creates a new database suitable for use in testing. It returns an
// established database connection and the configuration.
func (i *TestInstance) NewDatabase(tb testing.TB, cacher cache.Cacher, opts ...UtilOption) (*Database, *Config) {
	tb.Helper()

	// Ensure we should actually create the database.
	if i.skipReason != "" {
		tb.Skip(i.skipReason)
	}

	ctx := project.TestContext(tb)

	// Clone the template database.
	newDatabaseName, err := i.clone()
	if err != nil {
		tb.Fatal(err)
	}

	// Build the new connection to the new database name.
	config := i.db.config.clone()
	config.Name = newDatabaseName

	// Parse configuration and override with test data.
	db, err := config.Load(ctx)
	if err != nil {
		tb.Fatalf("failed to load database configuration: %s", err)
	}

	// Apply any options.
	for _, f := range opts {
		db, config = f(db, config)
	}

	if db.keyManager == nil {
		db.keyManager = keys.TestKeyManager(tb)
	}
	if db.config.EncryptionKey == "" {
		db.config.EncryptionKey = keys.TestEncryptionKey(tb, db.keyManager)
	}

	// Try to establish a connection to the database.
	if err := db.OpenWithCacher(ctx, cacher); err != nil {
		tb.Fatalf("failed to open database connection: %s", err)
	}
	db.db.SetLogger(gorm.Logger{LogWriter: log.New(ioutil.Discard, "", 0)})
	db.db.LogMode(false)

	// Close connection and delete database when done.
	tb.Cleanup(func() {
		// Close connection first. It is an error to drop a database with active
		// connections.
		if err := db.Close(); err != nil {
			tb.Errorf("failed to close database connection: %s", err)
		}

		// Drop the database to keep the container from running out of resources.
		q := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE);`, newDatabaseName)

		i.dbLock.Lock()
		defer i.dbLock.Unlock()

		if err := i.db.db.Exec(q).Error; err != nil {
			tb.Errorf("failed to drop database %q: %s", newDatabaseName, err)
		}
	})

	return db, config
}

// clone creates a new database with a random name from the template instance.
func (i *TestInstance) clone() (string, error) {
	// Generate a random database name.
	name, err := randomDatabaseName()
	if err != nil {
		return "", fmt.Errorf("failed to generate random database name: %w", err)
	}

	// Setup context and create SQL command. Unfortunately we cannot use parameter
	// injection here as that's only valid for prepared statements, for which this
	// is not. Fortunately both inputs can be trusted in this case.
	q := fmt.Sprintf(`CREATE DATABASE "%s" WITH TEMPLATE "%s";`, name, databaseName)

	// Unfortunately postgres does not allow parallel database creation from the
	// same template, so this is guarded with a lock.
	i.dbLock.Lock()
	defer i.dbLock.Unlock()

	// Clone the template database as the new random database name.
	if err := i.db.db.Exec(q).Error; err != nil {
		return "", fmt.Errorf("failed to clone template database: %w", err)
	}
	return name, nil
}

// runMigrations runs the migrations for the database.
func runMigrations(db *Database) error {
	if err := db.MigrateTo(context.Background(), "", false); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	return nil
}

// postgresRepo returns the postgres container image name based on an
// environment variable, or the default value if the environment variable is
// unset.
func postgresRepo() (string, string, error) {
	ref := os.Getenv("CI_POSTGRES_IMAGE")
	if ref == "" {
		ref = defaultPostgresImageRef
	}

	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid reference for database container: %q", ref)
	}
	return parts[0], parts[1], nil
}

// randomDatabaseName returns a random database name.
func randomDatabaseName() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateKeys creates qty keys of the specified length.
func generateKeys(qty, length int) ([]envconfig.Base64Bytes, error) {
	keys := make([]envconfig.Base64Bytes, 0, qty)
	for i := 0; i < qty; i++ {
		buf := make([]byte, length)
		n, err := rand.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read random: %w", err)
		}
		if n < length {
			return nil, fmt.Errorf("insufficient bytes read (-%d, +%d)", n, length)
		}
		keys = append(keys, buf)
	}

	return keys, nil
}
