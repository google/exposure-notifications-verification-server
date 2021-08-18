// Copyright 2021 the Exposure Notifications Verification Server authors
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

	"github.com/sethvargo/go-envconfig"
)

// MetricsRegistrarConfig represents the environment based configuration for the
// metrics registration server.
type MetricsRegistrarConfig struct {
	Observability observability.Config
	Features      FeatureConfig

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// Port is the port on which to bind.
	Port string `env:"PORT,default=8080"`
}

// NewMetricsRegistrarConfig returns the environment config for the metrics
// registration server. Only needs to be called once per instance, but may be
// called multiple times.
func NewMetricsRegistrarConfig(ctx context.Context) (*MetricsRegistrarConfig, error) {
	var config MetricsRegistrarConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *MetricsRegistrarConfig) Validate() error {
	return nil
}

func (c *MetricsRegistrarConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
