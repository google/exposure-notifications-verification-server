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
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/issue"
)

type Metrics struct {
	IssueAttempts       *stats.Int64Measure
	QuotaErrors         *stats.Int64Measure
	QuotaExceeded       *stats.Int64Measure
	CodesIssued         *stats.Int64Measure
	CodeIssueErrors     *stats.Int64Measure
	SMSSent             *stats.Int64Measure
	SMSSendErrors       *stats.Int64Measure
	RealmTokenIssued    *stats.Int64Measure
	RealmTokenRemaining *stats.Int64Measure
	RealmTokenCapacity  *stats.Float64Measure
}

func registerMetrics() (*Metrics, error) {
	mIssueAttempts := stats.Int64(MetricPrefix+"/attempts", "The number of attempts to issue codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/attempt_count",
		Measure:     mIssueAttempts,
		Description: "The count of the number of attempts to issue codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mQuotaErrors := stats.Int64(MetricPrefix+"/quota_errors", "The number of errors when taking from the limiter", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/quota_errors_count",
		Measure:     mQuotaErrors,
		Description: "The count of the number of errors to the limiter",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mQuotaExceeded := stats.Int64(MetricPrefix+"/quota_exceeded", "The number of times quota has been exceeded", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/quota_exceeded_count",
		Measure:     mQuotaExceeded,
		Description: "The count of the number of times quota has been exceeded",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mCodesIssued := stats.Int64(MetricPrefix+"/codes_issued", "The number of verification codes issued", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/codes_issued_count",
		Measure:     mCodesIssued,
		Description: "The count of verification codes issued",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mCodeIssueErrors := stats.Int64(MetricPrefix+"/code_issue_error", "The number of failed code issues", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_issue_error_count",
		Measure:     mCodeIssueErrors,
		Description: "The count of the number of times a code fails to issue",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mSMSSent := stats.Int64(MetricPrefix+"/sms_sent", "The number of SMS messages sent", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/sms_sent_count",
		Measure:     mSMSSent,
		Description: "The count of verification codes sent over SMS",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mSMSSendErrors := stats.Int64(MetricPrefix+"/sms_send_error", "The number of failed SMS sends", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/sms_send_error_count",
		Measure:     mSMSSendErrors,
		Description: "The count of the number of a code issue failed due to SMS send failure",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mRealmTokenRemaining := stats.Int64(MetricPrefix+"/realm_token_remaining", "Remaining number of verification codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/realm_token_remaining_latest",
		Description: "Latest realm remaining tokens",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Measure:     mRealmTokenRemaining,
		Aggregation: view.LastValue(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mRealmTokenIssued := stats.Int64(MetricPrefix+"/realm_token_issued", "Total issued verification codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/realm_token_issued_latest",
		Description: "Latest realm issued tokens",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Measure:     mRealmTokenIssued,
		Aggregation: view.LastValue(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	mRealmTokenCapacity := stats.Float64(MetricPrefix+"/realm_token_capacity", "Capacity utilization for issuing verification codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/realm_token_capacity_latest",
		Description: "Latest realm token capacity utilization",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Measure:     mRealmTokenCapacity,
		Aggregation: view.LastValue(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Metrics{
		IssueAttempts:       mIssueAttempts,
		QuotaErrors:         mQuotaErrors,
		QuotaExceeded:       mQuotaExceeded,
		CodesIssued:         mCodesIssued,
		CodeIssueErrors:     mCodesIssued,
		SMSSent:             mSMSSent,
		SMSSendErrors:       mSMSSendErrors,
		RealmTokenIssued:    mRealmTokenIssued,
		RealmTokenRemaining: mRealmTokenRemaining,
		RealmTokenCapacity:  mRealmTokenCapacity,
	}, nil
}
