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
	"fmt"
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
	Type CacherType `env:"TYPE, default=NOOP"`

	// Redis options
	RedisAddress  string `env:"REDIS_ADDRESS"`
	RedisUsername string `env:"REDIS_USERNAME"`
	RedisPassword string `env:"REDIS_PASSWORD"`
}

func CacherFor(ctx context.Context, prefix string, c *Config) (Cacher, error) {
	switch typ := c.Type; typ {
	case TypeNoop:
		return NewNoop()
	case TypeInMemory:
		return NewInMemory(&InMemoryConfig{
			Prefix: prefix,
		})
	case TypeRedis:
		return NewRedis(&RedisConfig{
			Prefix:   prefix,
			Address:  c.RedisAddress,
			Username: c.RedisUsername,
			Password: c.RedisPassword,
		})
	default:
		return nil, fmt.Errorf("unknown cacher type: %v", typ)
	}
}
