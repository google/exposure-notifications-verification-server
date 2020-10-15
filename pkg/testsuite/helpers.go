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

package testsuite

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/sethvargo/go-retry"
)

func randomString() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04x", n), nil
}

type prefixRoundTripper struct {
	addr string
	rt   http.RoundTripper
}

// RoundTrip wraps transport's RoutTrip and sets the scheme and host address.
func (p *prefixRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	u := r.URL
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if u.Host == "" {
		u.Host = p.addr
	}

	return p.rt.RoundTrip(r)
}

// Eventually retries the given function n times, sleeping 1s between each
// invocation. To mark an error as retryable, wrap it in retry.RetryableError.
// Non-retryable errors return immediately.
func Eventually(times uint64, interval time.Duration, f func() error) error {
	ctx := context.Background()
	b, err := retry.NewConstant(interval)
	if err != nil {
		return fmt.Errorf("failed to create retry: %w", err)
	}
	b = retry.WithMaxRetries(times, b)

	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		return f()
	}); err != nil {
		return fmt.Errorf("did not return ok after %d retries: %w", times, err)
	}
	return nil
}
