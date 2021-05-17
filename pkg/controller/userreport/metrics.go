// Copyright 2021 Google LLC
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

package userreport

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/user-report"

var (
	mUserReportNotAllowed = stats.Float64(metricPrefix+"/user_report_not_allowed", "requests where realm doesnt allow user report", stats.UnitDimensionless)
	mMissingNonce         = stats.Float64(metricPrefix+"/missing_nonce", "requests missing a nonce value", stats.UnitDimensionless)
	mInvalidNonce         = stats.Float64(metricPrefix+"/invalid_nonce", "requests with an invalid nonce", stats.UnitDimensionless)
	mMissingAgreement     = stats.Float64(metricPrefix+"/missing_agreement", "requests missing agreement acceptance", stats.UnitDimensionless)
	mWebhookError         = stats.Float64(metricPrefix+"/webhook_error", "failed to make upstream webhook request", stats.UnitDimensionless)
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/user_report_not_allowed",
			Measure:     mUserReportNotAllowed,
			Description: "Count of number of requests where a realm hasnt enabled user report",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/missing_nonce",
			Measure:     mMissingNonce,
			Description: "Count of number of requests missing a nonce",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/invalid_nonce",
			Measure:     mInvalidNonce,
			Description: "Count of number of requests with an invalid nonce",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/missing_agreement",
			Measure:     mMissingAgreement,
			Description: "Count of number of requests missing acceptance of the agreement",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/webhook_error",
			Measure:     mWebhookError,
			Description: "Count of number of requests with a non-200 response",
			TagKeys:     observability.APITagKeys(),
			Aggregation: view.Count(),
		},
	}...)
}
