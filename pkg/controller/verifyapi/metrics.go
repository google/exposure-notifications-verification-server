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

package verifyapi

import (
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/api/verify"

var (
	mCodeVerifyAttempts    = stats.Int64(metricPrefix+"/attempts", "The number of attempted code verifications", stats.UnitDimensionless)
	mCodeVerifyExpired     = stats.Int64(metricPrefix+"/code_expired", "The number of attempted claims on expired codes", stats.UnitDimensionless)
	mCodeVerifyCodeUsed    = stats.Int64(metricPrefix+"/code_used", "The number of attempted claims on already used codes", stats.UnitDimensionless)
	mCodeVerifyInvalid     = stats.Int64(metricPrefix+"/code_invalid", "The number of attempted claims on invalid codes", stats.UnitDimensionless)
	mCodeVerified          = stats.Int64(metricPrefix+"/code_verified", "The number of successfully claimed codes", stats.UnitDimensionless)
	mCodeVerificationError = stats.Int64(metricPrefix+"/error", "The number of other errors in code issue", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/attempt_count",
			Measure:     mCodeVerifyAttempts,
			Description: "The count of attempted code verifications",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/code_expired_count",
			Measure:     mCodeVerifyExpired,
			Description: "The count of attempted claims on expired verification codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/code_used_count",
			Measure:     mCodeVerifyCodeUsed,
			Description: "The count of attempted claims on an already used verification codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/code_invalid_count",
			Measure:     mCodeVerifyInvalid,
			Description: "The count of attempted claims on invalid verification codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/code_verified_count",
			Measure:     mCodeVerified,
			Description: "The count of successfully verified codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/error_count",
			Measure:     mCodeVerificationError,
			Description: "The count of errors issuing verification codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
	}...)
}
