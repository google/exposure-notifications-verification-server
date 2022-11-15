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
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/ratelimit"
)

// IssueAPIVars is an interface that represents what is needed of the verification
// code issue API.
type IssueAPIVars struct {
	CollisionRetryCount uint          `env:"COLLISION_RETRY_COUNT,default=6"`
	AllowedSymptomAge   time.Duration `env:"ALLOWED_PAST_SYMPTOM_DAYS,default=672h"` // 672h is 28 days.
	EnforceRealmQuotas  bool          `env:"ENFORCE_REALM_QUOTAS, default=true"`

	// RealmOverrideGeneratedSMS - if a realm is included in the list, and that realm
	// is allowed to use generated SMS; then the generate SMS field is ignored and
	// internal SMS dispatch is used instead.
	RealmOverrideGeneratedSMS []uint `env:"REALM_OVERRIDE_GENERATED_SMS"`

	// For EN Express, the link will be
	// https://[realm-region].[ENX_REDIRECT_DOMAIN]/v?c=[longcode]
	// This repository contains a redirect service that can be used for this purpose.
	ENExpressRedirectDomain string `env:"ENX_REDIRECT_DOMAIN"`
}

func (c *IssueAPIVars) Validate() error {
	fields := []struct {
		Var  time.Duration
		Name string
	}{
		{c.AllowedSymptomAge, "ALLOWED_PAST_SYMPTOM_DAYS"},
	}

	for _, f := range fields {
		if err := checkPositiveDuration(f.Var, f.Name); err != nil {
			return err
		}
	}

	c.ENExpressRedirectDomain = strings.ToLower(c.ENExpressRedirectDomain)

	return nil
}

type IssueAPIConfig interface {
	IssueConfig() *IssueAPIVars

	GetRateLimitConfig() *ratelimit.Config
	GetFeatureConfig() *FeatureConfig
	IsMaintenanceMode() bool

	// GetAuthenticatedSMSFailClosed indicates how the system should behave when
	// authenticated SMS fails.
	GetAuthenticatedSMSFailClosed() bool
}
