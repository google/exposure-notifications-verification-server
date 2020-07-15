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
	"net"
	"time"

	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-limiter/noopstore"
	"github.com/sethvargo/go-limiter/redisstore"
)

// RateLimitType represents a type of rate limiter.
type RateLimitType string

const (
	RateLimiterTypeNoop   RateLimitType = "NOOP"
	RateLimiterTypeMemory RateLimitType = "MEMORY"
	RateLimiterTypeRedis  RateLimitType = "REDIS"
)

// Config represents rate limiting configuration
type Config struct {
	// Common configuration
	Type     RateLimitType `env:"RATE_LIMIT_TYPE,default=NOOP"`
	Tokens   uint64        `env:"RATE_LIMIT_TOKENS,default=60"`
	Interval time.Duration `env:"RATE_LIMIT_INTERVAL,default=1m"`

	// Redis configuration
	RedisHost     string `env:"REDIS_HOST,default=127.0.0.1"`
	RedisPort     string `env:"REDIS_PORT,default=6379"`
	RedisUsername string `env:"REDIS_USERNAME"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisMinPool  uint64 `env:"REDIS_MIN_POOL,default=16"`
	RedisMaxPool  uint64 `env:"REDIS_MAX_POOL,default=64"`
}

// RateLimiterFor returns the rate limiter for the given type, or an error
// if one does not exist.
func RateLimiterFor(_ context.Context, c *Config) (limiter.Store, error) {
	switch c.Type {
	case RateLimiterTypeNoop:
		return noopstore.New()
	case RateLimiterTypeMemory:
		return memorystore.New(&memorystore.Config{
			Tokens:   c.Tokens,
			Interval: c.Interval,
		})
	case RateLimiterTypeRedis:
		addr := c.RedisHost + ":" + c.RedisPort
		return redisstore.New(&redisstore.Config{
			Tokens:          c.Tokens,
			Interval:        c.Interval,
			InitialPoolSize: c.RedisMinPool,
			MaxPoolSize:     c.RedisMaxPool,
			DialFunc: func() (net.Conn, error) {
				return net.Dial("tcp", addr)
			},
			AuthUsername: c.RedisUsername,
			AuthPassword: c.RedisPassword,
		})
	}

	return nil, fmt.Errorf("unknown rate limiter type: %v", c.Type)
}
