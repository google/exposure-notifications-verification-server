// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	redigo "github.com/opencensus-integrations/redigo/redis"
	"go.opencensus.io/stats/view"
)

var deletePrefixScript = redigo.NewScript(0,
	`local keys = redis.call("KEYS", ARGV[1]); return #keys > 0 and redis.call("UNLINK", unpack(keys)) or 0`)

var _ Cacher = (*redisCacher)(nil)

// redisCacher is a shared cache implementation backed by Redis. It's ideal for
// production installations since the cache is shared among all services.
type redisCacher struct {
	pool    *redigo.Pool
	keyFunc KeyFunc

	waitTimeout time.Duration

	stopped uint32
	stopCh  chan struct{}
}

type RedisConfig struct {
	// Address is the redis address and port. The default value is 127.0.0.1:6379.
	Address string

	// Username and Password are used for authentication.
	Username, Password string

	// IdleTimeout, MaxIdle, and MaxActive control connection handling.
	IdleTimeout time.Duration
	MaxIdle     int
	MaxActive   int

	// KeyFunc is the key function.
	KeyFunc KeyFunc

	// WaitTimeout is the maximum amount of time to wait for a connection to
	// become available.
	WaitTimeout time.Duration
}

// NewRedis creates a new in-memory cache.
func NewRedis(i *RedisConfig) (Cacher, error) {
	if i == nil {
		i = new(RedisConfig)
	}

	addr := "127.0.0.1:6379"
	if i.Address != "" {
		addr = i.Address
	}

	c := &redisCacher{
		pool: &redigo.Pool{
			Dial: func() (redigo.Conn, error) {
				return redigo.Dial("tcp", addr,
					redigo.DialPassword(i.Password))
			},
			TestOnBorrow: func(conn redigo.Conn, _ time.Time) error {
				_, err := conn.Do("PING")
				return err
			},

			IdleTimeout: i.IdleTimeout,
			MaxIdle:     i.MaxIdle,
			MaxActive:   i.MaxActive,
			Wait:        true,
		},
		keyFunc:     i.KeyFunc,
		waitTimeout: i.WaitTimeout,
		stopCh:      make(chan struct{}),
	}

	if err := view.Register(redigo.ObservabilityMetricViews...); err != nil {
		return nil, fmt.Errorf("redis view registration failure: %w", err)
	}

	return c, nil
}

// Fetch attempts to retrieve the given key from the cache. If successful, it
// returns the value. If the value does not exist, it calls f and caches the
// result of f in the cache for ttl. The ttl is calculated from the time the
// value is inserted, not the time the function is called.
func (c *redisCacher) Fetch(ctx context.Context, k *Key, out interface{}, ttl time.Duration, f FetchFunc) error {
	if c.isStopped() {
		return ErrStopped
	}

	key, err := k.Compute(c.keyFunc)
	if err != nil {
		return fmt.Errorf("failed to compute key: %w", err)
	}

	fn := func(conn redigo.ConnWithContext) (io.Reader, error) {
		cached, err := redigo.String(conn.DoContext(ctx, http.MethodGet, key))
		if err != nil && !errors.Is(err, redigo.ErrNil) {
			return nil, fmt.Errorf("failed to GET key: %w", err)
		}

		// Found a value
		if cached != "" {
			return strings.NewReader(cached), nil
		}

		// No value found
		if f == nil {
			return nil, ErrMissingFetchFunc
		}
		val, err := f()
		if err != nil {
			return nil, err
		}

		// Encode the value
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(val); err != nil {
			return nil, fmt.Errorf("failed to encode value: %w", err)
		}

		if _, err := conn.DoContext(ctx, "WATCH", key); err != nil {
			return nil, fmt.Errorf("failed to WATCH key: %w", err)
		}

		if _, err := conn.DoContext(ctx, "MULTI"); err != nil {
			return nil, fmt.Errorf("failed to MULTI: %w", err)
		}

		if _, err := conn.DoContext(ctx, "PSETEX", key, ttl.Milliseconds(), encoded.String()); err != nil {
			err = fmt.Errorf("failed to PSETEX: %w", err)

			if _, derr := conn.DoContext(ctx, "DISCARD"); derr != nil {
				err = fmt.Errorf("failed to DISCARD: %v, original error: %w", derr, err)
			}

			return nil, err
		}

		if _, err := conn.DoContext(ctx, "EXEC"); err != nil {
			return nil, fmt.Errorf("failed to EXEC: %w", err)
		}

		return &encoded, nil
	}

	// This is a CAS operation, so retry
	for i := 0; i < 5; i++ {
		err = c.withConn(func(c redigo.ConnWithContext) error {
			r, err := fn(c)
			if err != nil {
				return err
			}

			// Decode the value into out.
			if err := json.NewDecoder(r).Decode(out); err != nil {
				return fmt.Errorf("failed to decode value")
			}

			return nil
		})
		if err == nil {
			break
		}
	}

	return err
}

// Write adds a new item to the cache with the given TTL.
func (c *redisCacher) Write(ctx context.Context, k *Key, value interface{}, ttl time.Duration) error {
	if c.isStopped() {
		return ErrStopped
	}

	key, err := k.Compute(c.keyFunc)
	if err != nil {
		return fmt.Errorf("failed to compute key: %w", err)
	}

	return c.withConn(func(conn redigo.ConnWithContext) error {
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(value); err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}

		if _, err := redigo.String(conn.DoContext(ctx, "PSETEX", key, ttl.Milliseconds(), encoded.String())); err != nil {
			return fmt.Errorf("failed to PSETEX value: %w", err)
		}
		return nil
	})
}

// Read fetches the value at the key. If the value does not exist, it returns
// ErrNotFound.
func (c *redisCacher) Read(ctx context.Context, k *Key, out interface{}) error {
	if c.isStopped() {
		return ErrStopped
	}

	key, err := k.Compute(c.keyFunc)
	if err != nil {
		return fmt.Errorf("failed to compute key: %w", err)
	}

	return c.withConn(func(conn redigo.ConnWithContext) error {
		val, err := redigo.String(conn.DoContext(ctx, http.MethodGet, key))
		if err != nil && !errors.Is(err, redigo.ErrNil) {
			return fmt.Errorf("failed to GET value: %w", err)
		}
		if val == "" {
			return ErrNotFound
		}

		r := strings.NewReader(val)
		if err := json.NewDecoder(r).Decode(out); err != nil {
			return fmt.Errorf("failed to decode cached value: %w", err)
		}
		return nil
	})
}

// Delete removes an item from the cache, if it exists, regardless of TTL.
func (c *redisCacher) Delete(ctx context.Context, k *Key) error {
	if c.isStopped() {
		return ErrStopped
	}

	key, err := k.Compute(c.keyFunc)
	if err != nil {
		return fmt.Errorf("failed to compute key: %w", err)
	}

	return c.withConn(func(conn redigo.ConnWithContext) error {
		if _, err := conn.DoContext(ctx, "UNLINK", key); err != nil && !errors.Is(err, redigo.ErrNil) {
			return fmt.Errorf("failed to UNLINK: %w", err)
		}
		return nil
	})
}

// DeletePrefix removes all items that start with the given prefix.
func (c *redisCacher) DeletePrefix(ctx context.Context, prefix string) error {
	if c.isStopped() {
		return ErrStopped
	}

	search := prefix + "*"
	return c.withConn(func(conn redigo.ConnWithContext) error {
		if _, err := deletePrefixScript.Do(conn, search); err != nil {
			return fmt.Errorf("failed to delete prefix: %w", err)
		}
		return nil
	})
}

// Close completely stops the cacher. It is not safe to use after closing.
func (c *redisCacher) Close() error {
	if !atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
		return nil
	}

	close(c.stopCh)
	if err := c.pool.Close(); err != nil {
		return fmt.Errorf("failed to close pool: %w", err)
	}
	return nil
}

// withConn runs the function with a conn, ensuring cleanup of the connection.
func (c *redisCacher) withConn(f func(conn redigo.ConnWithContext) error) error {
	if f == nil {
		return fmt.Errorf("missing function")
	}

	ctx, done := context.WithTimeout(context.Background(), c.waitTimeout)
	defer done()

	conn, ok := c.pool.GetWithContext(ctx).(redigo.ConnWithContext)
	if !ok {
		return fmt.Errorf("redis conn is not ConnWithContext")
	}

	if err := conn.Err(); err != nil {
		return fmt.Errorf("connection is not usable: %w", err)
	}

	if err := f(conn); err != nil {
		if cerr := conn.CloseContext(ctx); cerr != nil {
			return fmt.Errorf("failed to close connection: %v, original error: %w", cerr, err)
		}
		return err
	}

	if err := conn.CloseContext(ctx); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}

// isStopped returns true if the cacher is stopped.
func (c *redisCacher) isStopped() bool {
	return atomic.LoadUint32(&c.stopped) == 1
}
