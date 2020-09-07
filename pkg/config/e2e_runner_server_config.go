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

package config

import (
	"context"

	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig"
)

// E2ERunnerConfig represents the environment based configuration for the e2e-runner server.
type E2ERunnerConfig struct {
	Database      database.Config
	Observability *observability.Config

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	Port string `env:"PORT,default=8080"`

	// Share config between server and command line versions.
	TestConfig E2ETestConfig
}

type E2ETestConfig struct {
	VerificationAdminAPIServer string `env:"VERIFICATION_ADMIN_API, default=http://localhost:8081"`
	VerificationAdminAPIKey    string `env:"VERIFICATION_ADMIN_API_KEY"`
	VerificationAPIServer      string `env:"VERIFICATION_SERVER_API, default=http://localhost:8082"`
	VerificationAPIServerKey   string `env:"VERIFICATION_SERVER_API_KEY"`
	KeyServer                  string `env:"KEY_SERVER, default=http://localhost:8080"`
	HealthAuthorityCode        string `env:"HEALTH_AUTHORITY_CODE,required"`
	DoRevise                   bool   `env:"DO_REVISIONS"`
}

// NewE2ERunnerConfig returns the environment config for the e2e-runner server.
// Only needs to be called once per instance, but may be called multiple times.
func NewE2ERunnerConfig(ctx context.Context) (*E2ERunnerConfig, error) {
	var config E2ERunnerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

// NewE2ETestConfig contains just the necessary elements for command line execution.
func NewE2ETestConfig(ctx context.Context) (*E2ETestConfig, error) {
	var config E2ETestConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *E2ERunnerConfig) Validate() error {
	return nil
}
