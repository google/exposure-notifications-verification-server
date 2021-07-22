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

package rotation

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const metricPrefix = observability.MetricRoot + "/rotation"

var (
	mClaimRequests       = stats.Int64(metricPrefix+"/claim_requests", "The number of rotation claim requests.", stats.UnitDimensionless)
	mLatencyMs           = stats.Float64(metricPrefix+"/requests", "The number of rotation requests.", stats.UnitMilliseconds)
	mSecretsSuccess      = stats.Int64(metricPrefix+"/secrets_success", "successful secrets rotation", stats.UnitDimensionless)
	mTokenSuccess        = stats.Int64(metricPrefix+"/token_success", "successful token rotation", stats.UnitDimensionless)
	mVerificationSuccess = stats.Int64(metricPrefix+"/verification_success", "successful verification rotation", stats.UnitDimensionless)

	itemTagKey = tag.MustNewKey("item")
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/requests_count",
			Measure:     mLatencyMs,
			Description: "The count of the rotation requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey, itemTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/requests_latency",
			Measure:     mLatencyMs,
			Description: "The latency distribution of the rotation requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey, itemTagKey),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/claim_requests_count",
			Measure:     mClaimRequests,
			Description: "The count of the rotation claim requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/secrets/success",
			Description: "Number of secrets rotation successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mSecretsSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/token/success",
			Description: "Number of token rotation successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mTokenSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/verification/success",
			Description: "Number of verification rotation successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mVerificationSuccess,
			Aggregation: view.Count(),
		},
	}...)
}
