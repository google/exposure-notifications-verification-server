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
	"sync/atomic"
	"time"
)

var _ Cacher = (*noop)(nil)

// noop is a full pass-through cache.
type noop struct {
	stopped uint32
}

// NewNoop creates a new noop cache.
func NewNoop() (Cacher, error) {
	return &noop{}, nil
}

// Fetch calls FetchFunc and does no caching.
func (c *noop) Fetch(_ context.Context, key string, out interface{}, ttl time.Duration, f FetchFunc) error {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return ErrStopped
	}
	if f == nil {
		return ErrMissingFetchFunc
	}
	val, err := f()
	if err != nil {
		return err
	}
	return readInto(val, out)
}

// Write does nothing.
func (c *noop) Write(_ context.Context, _ string, _ interface{}, ttl time.Duration) error {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return ErrStopped
	}
	return nil
}

// Read always returns ErrNotFound.
func (c *noop) Read(_ context.Context, _ string, _ interface{}) error {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return ErrStopped
	}
	return ErrNotFound
}

// Delete does nothing.
func (c *noop) Delete(_ context.Context, _ string) error {
	if atomic.LoadUint32(&c.stopped) == 1 {
		return ErrStopped
	}
	return nil
}

// Close stops the cacher.
func (c *noop) Close() error {
	if !atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		return nil
	}
	return nil
}
