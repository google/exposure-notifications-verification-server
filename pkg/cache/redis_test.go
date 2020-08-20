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

package cache

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/sethvargo/go-retry"
)

func TestRedis(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skipf("🚧 Skipping database tests (short)!")
	}

	if skip, _ := strconv.ParseBool(os.Getenv("SKIP_REDIS_TESTS")); skip {
		t.Skipf("🚧 Skipping database tests (SKIP_REDIS_TESTS is set)!")
	}

	ctx := context.Background()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal(err)
	}

	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "6-alpine",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure container is cleaned up.
	t.Cleanup(func() {
		if err := pool.Purge(container); err != nil {
			t.Errorf("failed to cleanup redis container: %v", err)
		}
	})

	// Get the host. On Mac, Docker runs in a VM.
	host := container.GetBoundIP("6379/tcp")
	port := container.GetPort("6379/tcp")

	// Create the cacher
	cacher, err := NewRedis(&RedisConfig{
		Address: host + ":" + port,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Close the cacher when done
	t.Cleanup(func() {
		if err := cacher.Close(); err != nil {
			t.Fatal(err)
		}
	})

	// Wait for the container to start - we'll retry connections in a loop below,
	// but there's no point in trying immediately.
	time.Sleep(1 * time.Second)

	// Wait for the container to be ready.
	b, err := retry.NewFibonacci(500 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	b = retry.WithMaxRetries(5, b)
	b = retry.WithCappedDuration(2*time.Second, b)

	if err := retry.Do(ctx, b, func(_ context.Context) error {
		if err := cacher.Delete(ctx, "foo"); err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Run tests
	exerciseCacher(t, cacher)
}
