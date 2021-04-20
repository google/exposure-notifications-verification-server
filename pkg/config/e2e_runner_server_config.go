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
	Features      FeatureConfig

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	Port string `env:"PORT,default=8080"`

	VerificationAdminAPIServer string `env:"VERIFICATION_ADMIN_API, default=http://localhost:8081"`
	VerificationAdminAPIKey    string `env:"VERIFICATION_ADMIN_API_KEY"`
	VerificationAPIServer      string `env:"VERIFICATION_SERVER_API, default=http://localhost:8082"`
	VerificationAPIServerKey   string `env:"VERIFICATION_SERVER_API_KEY"`
	KeyServer                  string `env:"KEY_SERVER, default=http://localhost:8080"`
	HealthAuthorityCode        string `env:"HEALTH_AUTHORITY_CODE,required"`
	// Not environment vars, but set by each type of test run.
	DoRevise     bool
	DoUserReport bool

	// ENXRedirectURL is the host to use for testing the ENX redirector service.
	// This should be the value of the e2e realm's host, like
	// "https://e2e-realm.redirect-domain.com", where "redirect-domain.com" is
	// your enx redirect domain. The protocol is required. If this value is blank,
	// the enx redirect tests are not executed on the e2e-runner.
	ENXRedirectURL string `env:"ENX_REDIRECT_URL"`
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
