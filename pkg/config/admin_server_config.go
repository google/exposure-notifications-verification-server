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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
)

var _ IssueAPIConfig = (*AdminAPIServerConfig)(nil)

// AdminAPIServerConfig represents the environment based config for the Admin API Server.
type AdminAPIServerConfig struct {
	Database database.Config

	Port                int           `env:"PORT,default=8080"`
	RateLimit           uint64        `env:"RATE_LIMIT,default=60"`
	APIKeyCacheDuration time.Duration `env:"API_KEY_CACHE_DURATION,default=5m"`

	CodeDuration        time.Duration `env:"CODE_DURATION,default=1h"`
	CodeDigits          uint          `env:"CODE_DIGITS,default=8"`
	CollisionRetryCount uint          `env:"COLISSION_RETRY_COUNT,default=6"`
	AllowedSymptomAge   time.Duration `env:"ALLOWED_PAST_SYMPTOM_DAYS,default=336h"` // 336h is 14 days.
}

// NewAdminAPIServerConfig returns the environment config for the Admin API server.
// Only needs to be called once per instance, but may be called multiple times.
func NewAdminAPIServerConfig(ctx context.Context) (*AdminAPIServerConfig, error) {
	var config AdminAPIServerConfig
	if err := ProcessWith(ctx, &config, envconfig.OsLookuper()); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *AdminAPIServerConfig) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.APIKeyCacheDuration, "API_KEY_CACHE_DURATION"},
		{c.AllowedSymptomAge, "ALLOWED_PAST_SYMPTOM_DAYS"},
		{c.CodeDuration, "CODE_DURATION"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	return nil
}

func (c *AdminAPIServerConfig) GetColissionRetryCount() uint {
	return c.CollisionRetryCount
}

func (c *AdminAPIServerConfig) GetAllowedSymptomAge() time.Duration {
	return c.AllowedSymptomAge
}

func (c *AdminAPIServerConfig) GetVerificationCodeDuration() time.Duration {
	return c.CodeDuration
}

func (c *AdminAPIServerConfig) GetVerficationCodeDigits() uint {
	return c.CodeDigits
}
