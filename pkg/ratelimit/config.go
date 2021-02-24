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

// Package ratelimit defines common rate limiting logic and config.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/redis"
	redigo "github.com/opencensus-integrations/redigo/redis"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-limiter/noopstore"
	"github.com/sethvargo/go-redisstore"
	"go.opencensus.io/trace"
)

// Type represents a type of rate limiter.
type Type string

const (
	RateLimiterTypeNoop   Type = "NOOP"
	RateLimiterTypeMemory Type = "MEMORY"
	RateLimiterTypeRedis  Type = "REDIS"
)

// Config represents rate limiting configuration
type Config struct {
	// Common configuration
	Type     Type          `env:"RATE_LIMIT_TYPE, default=NOOP"`
	Tokens   uint64        `env:"RATE_LIMIT_TOKENS, default=120"`
	Interval time.Duration `env:"RATE_LIMIT_INTERVAL, default=1m"`

	// HMACKey is the key to use when calculating the HMAC of keys before saving
	// them in the rate limiter.
	HMACKey envconfig.Base64Bytes `env:"RATE_LIMIT_HMAC_KEY, required"`

	// Redis configuration
	Redis redis.Config `env:",prefix=RATE_LIMIT_"`
}

// RateLimiterFor returns the rate limiter for the given type, or an error
// if one does not exist.
func RateLimiterFor(ctx context.Context, c *Config) (limiter.Store, error) {
	switch c.Type {
	case RateLimiterTypeNoop:
		return noopstore.New()
	case RateLimiterTypeMemory:
		return memorystore.New(&memorystore.Config{
			Tokens:   c.Tokens,
			Interval: c.Interval,
		})
	case RateLimiterTypeRedis:
		addr := c.Redis.Host + ":" + c.Redis.Port

		config := &redisstore.Config{
			Tokens:   c.Tokens,
			Interval: c.Interval,
		}

		return redisstore.NewWithPool(config, &redigo.Pool{
			Dial: func() (redigo.Conn, error) {
				options := redigo.TraceOptions{}
				// set default attributes
				redigo.WithDefaultAttributes(trace.StringAttribute("span.type", "DB"))(&options)

				return redigo.DialWithContext(ctx, "tcp", addr,
					redigo.DialPassword(c.Redis.Password),
					redigo.DialTraceOptions(options),
				)
			},
			TestOnBorrow: func(conn redigo.Conn, _ time.Time) error {
				_, err := conn.Do("PING")
				return err
			},

			IdleTimeout: c.Redis.IdleTimeout,
			MaxIdle:     c.Redis.MaxIdle,
			MaxActive:   c.Redis.MaxActive,
		})
	}

	return nil, fmt.Errorf("unknown rate limiter type: %v", c.Type)
}
