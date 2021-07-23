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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/sethvargo/go-envconfig"
)

// StatsPullerConfig represents the environment-based configuration for the
// stats-puller service.
type StatsPullerConfig struct {
	Database      database.Config
	Observability observability.Config
	Features      FeatureConfig

	// Certificate signing
	CertificateSigning CertificateSigningConfig

	// KeyServerURL is the default URL of the key server - individual realms may override it
	KeyServerURL string `env:"KEY_SERVER_URL, required"`
	// The audience value to send to the keyserver.
	// Default matches: https://github.com/google/exposure-notifications-server/blob/main/internal/verification/config.go
	KeyServerStatsAudience string        `env:"KEY_SERVER_STATS_AUDIENCE, default=keyserver"`
	FileSizeLimitBytes     int64         `env:"STATS_PULLER_SIZE_LIMIT, default=64000"`
	DownloadTimeout        time.Duration `env:"STATS_PULLER_DOWNLOAD_TIMEOUT, default=1m"`

	// Port is the port upon which to bind.
	Port string `env:"PORT, default=8080"`

	// DevMode produces additional debugging information. Do not enable in
	// production environments.
	DevMode bool `env:"DEV_MODE"`

	// MinTTL is the minimum amount of time that must elapse between attempting
	// stats-pull events. This is used to control whether the pull is actually
	// attempted at the controller layer, independent of the data layer. In
	// effect, it rate limits the number of rotation requests.
	MinTTL time.Duration `env:"MIN_TTL, default=5m"`

	// StatsPullerMinPeriod defines the period for which the stats puller will hold a lock
	// which prevents other calls from entering.
	StatsPullerMinPeriod time.Duration `env:"STATS_PULLER_MIN_PERIOD, default=5m"`

	// MaxWorkers is the maximum number of parallel workers to use when pulling
	// statistics. The value must be greater than 0.
	MaxWorkers int64 `env:"STATS_PULLER_MAX_WORKERS, default=5"`
}

// NewStatsPullerConfig returns the config for the stats-puller service.
func NewStatsPullerConfig(ctx context.Context) (*StatsPullerConfig, error) {
	var config StatsPullerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *StatsPullerConfig) ObservabilityExporterConfig() *observability.Config {
	return &c.Observability
}
