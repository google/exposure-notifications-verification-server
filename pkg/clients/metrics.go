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

package clients

import (
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const metricPrefix = observability.MetricRoot + "/e2e"

var (
	mLatencyMs = stats.Int64(metricPrefix+"/request", "request latency", stats.UnitDimensionless)

	// The name of step in e2e test.
	stepTagKey = tag.MustNewKey("step")

	// The type of the e2e test.
	testTypeTagKey = tag.MustNewKey("test_type")
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/request_count",
			Measure:     mLatencyMs,
			Description: "Count of e2e requests",
			TagKeys:     append(observability.CommonTagKeys(), observability.ResultTagKey, stepTagKey, testTypeTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/request_latency",
			Measure:     mLatencyMs,
			Description: "Distribution of e2e requests latency in ms",
			TagKeys:     append(observability.CommonTagKeys(), stepTagKey, testTypeTagKey),
			Aggregation: view.Distribution(0, 0.1, 1, 10, 100, 1000, 10000, 100000),
		},
	}...)
}
