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

package config

import (
	"context"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// Modeler is the configuration for the modeler service.
type Modeler struct {
	Cache         cache.Config
	Database      database.Config
	Observability observability.Config
	RateLimit     ratelimit.Config

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	Port string `env:"PORT, default=8080"`

	// MinValue and MaxValue determine the floor and ceiling limits for the
	// modeler.
	MinValue uint `env:"MODELER_MIN_VALUE, default=10"`
	MaxValue uint `env:"MODELER_MAX_VALUE, default=20000"`
}

// NewModeler returns the config for the modeler server.
func NewModeler(ctx context.Context) (*Modeler, error) {
	var config Modeler
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Modeler) Validate() error {
	return nil
}

func (c *Modeler) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
