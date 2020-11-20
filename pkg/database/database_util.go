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
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jinzhu/gorm"
	"github.com/ory/dockertest"
	"github.com/sethvargo/go-envconfig"
)

var (
	approxTime = cmp.Options{cmpopts.EquateApproxTime(time.Second)}
)

// NewTestDatabaseWithCacher creates a database configured with a cacher for use
// in testing.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestDatabaseWithCacher(tb testing.TB, cacher cache.Cacher) (*Database, *Config) {
	tb.Helper()

	if testing.Short() {
		tb.Skipf("🚧 Skipping database tests (short)!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_DATABASE_TESTS")); skip {
		tb.Skipf("🚧 Skipping database tests (SKIP_DATABASE_TESTS is set)!")
	}

	// Context.
	ctx := context.Background()

	// Create the pool (docker instance).
	pool, err := dockertest.NewPool("")
	if err != nil {
		tb.Fatalf("failed to create Docker pool: %s", err)
	}

	// Determine the container image to use.
	repo, tag := postgresRepo(tb)

	// These credentials (and this entire file) are only used for tests. They are
	// used when spinning up an in-memory database for tests.
	dbname, username, password := "en-verification-server", "my-username", "abcd1234"

	// Start the container.
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: repo,
		Tag:        tag,
		Env: []string{
			"LANG=C",
			"POSTGRES_DB=" + dbname,
			"POSTGRES_USER=" + username,
			"POSTGRES_PASSWORD=" + password,
		},
	})
	if err != nil {
		tb.Fatalf("failed to start postgres container: %s", err)
	}

	// Force the database container to stop.
	if err := container.Expire(30); err != nil {
		tb.Fatalf("failed to force-stop container: %v", err)
	}

	// Ensure container is cleaned up.
	tb.Cleanup(func() {
		if err := pool.Purge(container); err != nil {
			tb.Fatalf("failed to cleanup postgres container: %s", err)
		}
	})

	// Get the host. On Mac, Docker runs in a VM.
	host := container.GetBoundIP("5432/tcp")
	port := container.GetPort("5432/tcp")

	// build database config.
	config := &Config{
		APIKeyDatabaseHMAC:           generateKeys(tb, 3, 128),
		APIKeySignatureHMAC:          generateKeys(tb, 3, 128),
		VerificationCodeDatabaseHMAC: generateKeys(tb, 3, 128),

		User:     username,
		Port:     port,
		Host:     host,
		Name:     dbname,
		Password: password,
		SSLMode:  "disable",
		Secrets: secrets.Config{
			SecretManagerType: secrets.SecretManagerTypeInMemory,
		},

		Keys: keys.Config{
			KeyManagerType: keys.KeyManagerTypeFilesystem,
		},
	}

	// Wait for the container to start - we'll retry connections in a loop below,
	// but there's no point in trying immediately.
	time.Sleep(1 * time.Second)

	// Load the configuration
	db, err := config.Load(ctx)
	if err != nil {
		tb.Fatal(err)
	}

	db.keyManager = keys.TestKeyManager(tb)
	db.config.EncryptionKey = keys.TestEncryptionKey(tb, db.keyManager)

	if err := db.OpenWithCacher(ctx, cacher); err != nil {
		tb.Fatal(err)
	}

	// Disable logging temporarily for migrations. The callback registration is
	// really quite chatty.
	db.db.SetLogger(gorm.Logger{LogWriter: log.New(ioutil.Discard, "", 0)})
	db.db = db.db.LogMode(false)

	if err := db.RunMigrations(ctx); err != nil {
		tb.Fatalf("failed to migrate database: %v", err)
	}

	// Re-enable logging.
	db.db.SetLogger(gorm.Logger{LogWriter: log.New(os.Stdout, "", 0)})

	// Close db when done.
	tb.Cleanup(func() {
		if err := db.db.Close(); err != nil {
			tb.Fatal(err)
		}
	})

	return db, config
}

// NewTestDatabaseWithConfig creates a new database suitable for use in testing.
// This should not be used outside of testing, but it is exposed in the main
// package so it can be shared with other packages.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestDatabaseWithConfig(tb testing.TB) (*Database, *Config) {
	tb.Helper()

	return NewTestDatabaseWithCacher(tb, nil)
}

// NewTestDatabase creates a new test database with the defautl configuration.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestDatabase(tb testing.TB) *Database {
	tb.Helper()

	db, _ := NewTestDatabaseWithConfig(tb)
	return db
}

func generateKeys(tb testing.TB, qty, length int) []envconfig.Base64Bytes {
	tb.Helper()

	keys := make([]envconfig.Base64Bytes, 0, qty)
	for i := 0; i < qty; i++ {
		buf := make([]byte, length)
		n, err := rand.Read(buf)
		if err != nil {
			tb.Fatal(err)
		}
		if n < length {
			tb.Fatalf("insufficient bytes read: %v, expected %v", n, length)
		}
		keys = append(keys, buf)
	}

	return keys
}

func postgresRepo(tb testing.TB) (string, string) {
	postgresImageRef := os.Getenv("CI_POSTGRES_IMAGE")
	if postgresImageRef == "" {
		postgresImageRef = "postgres:13-alpine"
	}

	parts := strings.SplitN(postgresImageRef, ":", 2)
	if len(parts) != 2 {
		tb.Fatalf("invalid postgres ref %v", postgresImageRef)
	}
	return parts[0], parts[1]
}
