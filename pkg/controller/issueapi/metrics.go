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
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/api/issue"

var (
	mIssueAttempts       = stats.Int64(metricPrefix+"/attempts", "The number of attempts to issue codes", stats.UnitDimensionless)
	mQuotaErrors         = stats.Int64(metricPrefix+"/quota_errors", "The number of errors when taking from the limiter", stats.UnitDimensionless)
	mQuotaExceeded       = stats.Int64(metricPrefix+"/quota_exceeded", "The number of times quota has been exceeded", stats.UnitDimensionless)
	mCodesIssued         = stats.Int64(metricPrefix+"/codes_issued", "The number of verification codes issued", stats.UnitDimensionless)
	mCodeIssueErrors     = stats.Int64(metricPrefix+"/code_issue_error", "The number of failed code issues", stats.UnitDimensionless)
	mSMSSent             = stats.Int64(metricPrefix+"/sms_sent", "The number of SMS messages sent", stats.UnitDimensionless)
	mSMSSendErrors       = stats.Int64(metricPrefix+"/sms_send_error", "The number of failed SMS sends", stats.UnitDimensionless)
	mRealmTokenRemaining = stats.Int64(metricPrefix+"/realm_token_remaining", "Remaining number of verification codes", stats.UnitDimensionless)
	mRealmTokenIssued    = stats.Int64(metricPrefix+"/realm_token_issued", "Total issued verification codes", stats.UnitDimensionless)
	mRealmTokenCapacity  = stats.Float64(metricPrefix+"/realm_token_capacity", "Capacity utilization for issuing verification codes", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/attempt_count",
			Measure:     mIssueAttempts,
			Description: "The count of the number of attempts to issue codes",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/quota_errors_count",
			Measure:     mQuotaErrors,
			Description: "The count of the number of errors to the limiter",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/quota_exceeded_count",
			Measure:     mQuotaExceeded,
			Description: "The count of the number of times quota has been exceeded",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/codes_issued_count",
			Measure:     mCodesIssued,
			Description: "The count of verification codes issued",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/code_issue_error_count",
			Measure:     mCodeIssueErrors,
			Description: "The count of the number of times a code fails to issue",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/sms_sent_count",
			Measure:     mSMSSent,
			Description: "The count of verification codes sent over SMS",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/sms_send_error_count",
			Measure:     mSMSSendErrors,
			Description: "The count of the number of a code issue failed due to SMS send failure",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		}, {
			Name:        metricPrefix + "/realm_token_remaining_latest",
			Description: "Latest realm remaining tokens",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mRealmTokenRemaining,
			Aggregation: view.LastValue(),
		}, {
			Name:        metricPrefix + "/realm_token_issued_latest",
			Description: "Latest realm issued tokens",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mRealmTokenIssued,
			Aggregation: view.LastValue(),
		}, {
			Name:        metricPrefix + "/realm_token_capacity_latest",
			Description: "Latest realm token capacity utilization",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mRealmTokenCapacity,
			Aggregation: view.LastValue(),
		},
	}...)
}
