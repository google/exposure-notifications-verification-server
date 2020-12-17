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

// Package issuemetric implements the metrics for code issuance
package issuemetric

import (
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/api/issue"

var (
	RealmTokenUsed = stats.Int64(metricPrefix+"/realm_token_used", "# of realm token used.", stats.UnitDimensionless)
	SMSLatencyMs   = stats.Float64(metricPrefix+"/sms_request", "# of sms requests", stats.UnitMilliseconds)
	LatencyMs      = stats.Float64(metricPrefix+"/request", "# of code issue requests", stats.UnitMilliseconds)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/request_count",
			Measure:     LatencyMs,
			Description: "Count of code issue requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/request_latency",
			Measure:     LatencyMs,
			Description: "The latency distribution of code issue requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/sms_request_count",
			Measure:     SMSLatencyMs,
			Description: "The # of SMS requests",
			TagKeys:     append(observability.CommonTagKeys(), observability.ResultTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/sms_request_latency",
			Measure:     SMSLatencyMs,
			Description: "The # of SMS requests",
			TagKeys:     append(observability.CommonTagKeys(), observability.ResultTagKey),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/realm_token_used_count",
			Description: "The count of # of realm token used.",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     RealmTokenUsed,
			Aggregation: view.Count(),
		},
	}...)
}
