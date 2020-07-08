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
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ory/dockertest"
	"github.com/sethvargo/go-retry"
)

var (
	approxTime = cmp.Options{cmpopts.EquateApproxTime(time.Second)}
)

// NewTestDatabaseWithConfig creates a new database suitable for use in testing.
// This should not be used outside of testing, but it is exposed in the main
// package so it can be shared with other packages.
//
// All database tests can be skipped by running `go test -short` or by setting
// the `SKIP_DATABASE_TESTS` environment variable.
func NewTestDatabaseWithConfig(tb testing.TB) (*Database, *Config) {
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

	// Start the container.
	dbname, username, password := "en-verification-server", "my-username", "abcd1234"
	tb.Log("Starting database")
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "12-alpine",
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
	config := Config{
		User:     username,
		Port:     port,
		Host:     host,
		Name:     dbname,
		Password: password,
		SSLMode:  "disable",
	}

	// Wait for the container to start - we'll retry connections in a loop below,
	// but there's no point in trying immediately.
	time.Sleep(1 * time.Second)

	// Establish a connection to the database. Use a Fibonacci backoff instead of
	// exponential so wait times scale appropriately.
	b, err := retry.NewFibonacci(500 * time.Millisecond)
	if err != nil {
		tb.Fatalf("failed to configure backoff: %v", err)
	}
	b = retry.WithMaxRetries(10, b)
	b = retry.WithCappedDuration(10*time.Second, b)

	var db *Database
	if err := retry.Do(ctx, b, func(_ context.Context) error {
		var err error
		db, err = config.Open()
		if err != nil {
			tb.Logf("retrying error: %v", err)
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		tb.Fatalf("failed to start postgres: %s", err)
	}

	if err := db.RunMigrations(ctx); err != nil {
		tb.Fatalf("failed to migrate database: %v", err)
	}

	// Close db when done.
	tb.Cleanup(func() {
		db.db.Close()
	})

	return db, &config
}

func NewTestDatabase(tb testing.TB) *Database {
	tb.Helper()

	db, _ := NewTestDatabaseWithConfig(tb)
	return db
}
