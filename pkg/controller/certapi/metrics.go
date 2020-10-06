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

package certapi

import (
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	enobservability "github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/api/certificate"

var (
	mAttempts          = stats.Int64(metricPrefix+"/attempts", "certificate issue attempts", stats.UnitDimensionless)
	mTokenExpired      = stats.Int64(metricPrefix+"/token_expired", "expired tokens on certificate issue", stats.UnitDimensionless)
	mTokenUsed         = stats.Int64(metricPrefix+"/token_used", "already used tokens on certificate issue", stats.UnitDimensionless)
	mTokenInvalid      = stats.Int64(metricPrefix+"/invalid_token", "invalid tokens on certificate issue", stats.UnitDimensionless)
	mCertificateIssued = stats.Int64(metricPrefix+"/issue", "certificates issued", stats.UnitDimensionless)
	mCertificateErrors = stats.Int64(metricPrefix+"/errors", "certificate issue errors", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/attempt_count",
			Measure:     mAttempts,
			Description: "The count of certificate issue attempts",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/token_expired_count",
			Measure:     mTokenExpired,
			Description: "The count of expired tokens on certificate issue",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/token_used_count",
			Measure:     mTokenUsed,
			Description: "The count of already used tokens on certificate issue",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/invalid_token_count",
			Measure:     mTokenInvalid,
			Description: "The count of invalid tokens on certificate issue",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/issue_count",
			Measure:     mCertificateIssued,
			Description: "The count of certificates issued",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/error_count",
			Measure:     mCertificateErrors,
			Description: "The count of certificate issue errors",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
	}...)
}
