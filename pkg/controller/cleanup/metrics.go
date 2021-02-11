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

package cleanup

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const metricPrefix = observability.MetricRoot + "/cleanup"

var (
	mLatencyMs     = stats.Float64(metricPrefix+"/requests", "The number of cleanup requests.", stats.UnitMilliseconds)
	mClaimRequests = stats.Int64(metricPrefix+"/claim_requests", "The number of cleanup claim requests.", stats.UnitDimensionless)
)

var (
	// itemTagKey indicating what type of items is cleaned up in this step.
	// Potential values:
	// VERIFICATION_CODE
	// VERIFICATION_TOKEN
	// MOBILE_APP
	// AUDIT_ENTRY
	itemTagKey = tag.MustNewKey("item")
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/requests_count",
			Measure:     mLatencyMs,
			Description: "The count of the cleanup requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey, itemTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/requests_latency",
			Measure:     mLatencyMs,
			Description: "The latency distribution of the cleanup requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey, itemTagKey),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/claim_requests_count",
			Measure:     mClaimRequests,
			Description: "The count of the cleanup claim requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
			Aggregation: view.Count(),
		},
	}...)

}
