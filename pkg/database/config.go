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
	"fmt"
	"strings"
)

// Config represents the env var based configuration for database connections.
type Config struct {
	Name              string `env:"DB_NAME" json:",omitempty"`
	User              string `env:"DB_USER" json:",omitempty"`
	Host              string `env:"DB_HOST, default=localhost" json:",omitempty"`
	Port              string `env:"DB_PORT, default=5432" json:",omitempty"`
	SSLMode           string `env:"DB_SSLMODE, default=require" json:",omitempty"`
	ConnectionTimeout uint   `env:"DB_CONNECT_TIMEOUT" json:",omitempty"`
	Password          string `env:"DB_PASSWORD" json:"-"` // ignored by zap's JSON formatter
	SSLCertPath       string `env:"DB_SSLCERT" json:",omitempty"`
	SSLKeyPath        string `env:"DB_SSLKEY" json:",omitempty"`
	SSLRootCertPath   string `env:"DB_SSLROOTCERT" json:",omitempty"`
}

// ConnectionString returns the postgresql connection string based on this config.
//
// While this package could be adapted to different databases easily, this file
// and method in particular would need to change.
func (c *Config) ConnectionString() string {
	vals := dbValues(c)
	var p []string
	for k, v := range vals {
		p = append(p, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(p, " ")
}

func dbValues(config *Config) map[string]string {
	p := map[string]string{}
	setIfNotEmpty(p, "dbname", config.Name)
	setIfNotEmpty(p, "user", config.User)
	setIfNotEmpty(p, "host", config.Host)
	setIfNotEmpty(p, "port", config.Port)
	setIfNotEmpty(p, "sslmode", config.SSLMode)
	setIfPositive(p, "connect_timeout", config.ConnectionTimeout)
	setIfNotEmpty(p, "password", config.Password)
	setIfNotEmpty(p, "sslcert", config.SSLCertPath)
	setIfNotEmpty(p, "sslkey", config.SSLKeyPath)
	setIfNotEmpty(p, "sslrootcert", config.SSLRootCertPath)
	return p
}

func setIfNotEmpty(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}

func setIfPositive(m map[string]string, key string, val uint) {
	if val > 0 {
		m[key] = fmt.Sprintf("%d", val)
	}
}
