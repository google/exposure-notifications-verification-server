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

// Package redis defines redis-specific configurations.
package redis

import (
	"time"
)

// Config defines the default redis configuration and env processing.
type Config struct {
	Host     string `env:"REDIS_HOST, default=127.0.0.1"`
	Port     string `env:"REDIS_PORT, default=6379"`
	Username string `env:"REDIS_USERNAME"`
	Password string `env:"REDIS_PASSWORD"`

	IdleTimeout time.Duration `env:"REDIS_IDLE_TIMEOUT, default=1m"`
	MaxIdle     int           `env:"REDIS_MAX_IDLE, default=16"`
	MaxActive   int           `env:"REDIS_MAX_ACTIVE, default=64"`

	// WaitTimeout is the maximum amount of time to wait for a connection to
	// become available.
	WaitTimeout time.Duration `env:"REDIS_WAIT_TIMEOUT, default=1s"`
}
