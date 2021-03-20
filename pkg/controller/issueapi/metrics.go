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

package issueapi

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/api/issue"

const userReportMetricPrefix = observability.MetricRoot + "/api/userreport"

var (
	mLatencyMs = stats.Float64(metricPrefix+"/request", "# of code issue requests", stats.UnitMilliseconds)

	mAuthenticatedSMSFailure = stats.Int64(metricPrefix+"/authenticated_sms_failure", "# of failed attempts to sign authenticated sms", stats.UnitDimensionless)

	mSMSLatencyMs = stats.Float64(metricPrefix+"/sms_request", "# of sms requests", stats.UnitMilliseconds)

	mRealmTokenUsed = stats.Int64(metricPrefix+"/realm_token_used", "# of realm token used.", stats.UnitDimensionless)

	// separate metrics related to user report API.
	mUserReportLatencyMs = stats.Float64(userReportMetricPrefix+"/request", "verify requests latency", stats.UnitMilliseconds)

	mUserReportColission = stats.Int64(userReportMetricPrefix+"/phone_colission", "# of attempts to use a phone number multiple times for self report", stats.UnitDimensionless)
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/request_count",
			Measure:     mLatencyMs,
			Description: "Count of code issue requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/request_latency",
			Measure:     mLatencyMs,
			Description: "The latency distribution of code issue requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/authenticated_sms_failure_count",
			Measure:     mAuthenticatedSMSFailure,
			Description: "Count of failed attempts to sign authenticated SMS",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/sms_request_count",
			Measure:     mSMSLatencyMs,
			Description: "The # of SMS requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/sms_request_latency",
			Measure:     mSMSLatencyMs,
			Description: "The # of SMS requests",
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        metricPrefix + "/realm_token_used_count",
			Description: "The count of # of realm token used.",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mRealmTokenUsed,
			Aggregation: view.Count(),
		},
		{
			Name:        userReportMetricPrefix + "/request_count",
			Measure:     mUserReportLatencyMs,
			Description: "Count of user report requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        userReportMetricPrefix + "/request_latency",
			Measure:     mUserReportLatencyMs,
			Description: "Latency distribution of user report requests",
			TagKeys:     observability.APITagKeys(),
			Aggregation: ochttp.DefaultLatencyDistribution,
		},
		{
			Name:        userReportMetricPrefix + "/phone_colission",
			Description: "The count of # phone number colissions on user initiated report.",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mUserReportColission,
			Aggregation: view.Count(),
		},
	}...)
}
