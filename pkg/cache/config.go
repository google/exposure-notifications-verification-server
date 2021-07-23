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
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/redis"
	"github.com/sethvargo/go-envconfig"
)

// CacherType represents a type of cacher.
type CacherType string

const (
	TypeNoop     CacherType = "NOOP"
	TypeInMemory CacherType = "IN_MEMORY"
	TypeRedis    CacherType = "REDIS"
)

// Config represents configuration for a cacher.
type Config struct {
	Type CacherType `env:"CACHE_TYPE, default=IN_MEMORY"`

	// HMACKey is the hash key to use when for keys in the cacher.
	HMACKey envconfig.Base64Bytes `env:"CACHE_HMAC_KEY, required"`

	// Redis configuration
	Redis redis.Config `env:", prefix=CACHE_"`
}

func CacherFor(ctx context.Context, c *Config, keyFunc KeyFunc) (Cacher, error) {
	switch typ := c.Type; typ {
	case TypeNoop:
		return NewNoop()
	case TypeInMemory:
		return NewInMemory(&InMemoryConfig{
			KeyFunc: keyFunc,
		})
	case TypeRedis:
		return NewRedis(&RedisConfig{
			KeyFunc:     keyFunc,
			Address:     fmt.Sprintf("%s:%s", c.Redis.Host, c.Redis.Port),
			Username:    c.Redis.Username,
			Password:    c.Redis.Password,
			IdleTimeout: c.Redis.IdleTimeout,
			MaxIdle:     c.Redis.MaxIdle,
			MaxActive:   c.Redis.MaxActive,
			WaitTimeout: c.Redis.WaitTimeout,
		})
	default:
		return nil, fmt.Errorf("unknown cacher type: %v", typ)
	}
}
