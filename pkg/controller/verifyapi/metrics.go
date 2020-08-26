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
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/verify"
)

type Metrics struct {
	CodeVerifyAttempts    *stats.Int64Measure
	CodeVerifyExpired     *stats.Int64Measure
	CodeVerifyCodeUsed    *stats.Int64Measure
	CodeVerifyInvalid     *stats.Int64Measure
	CodeVerified          *stats.Int64Measure
	CodeVerificationError *stats.Int64Measure
}

func registerMetrics() (*Metrics, error) {
	mCodeVerifyAttempts := stats.Int64(MetricPrefix+"/attempts", "The number of attempted code verifications", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/attempt_count",
		Measure:     mCodeVerifyAttempts,
		Description: "The count of attempted code verifications",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerifyExpired := stats.Int64(MetricPrefix+"/code_expired", "The number of attempted claims on expired codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_expired_count",
		Measure:     mCodeVerifyExpired,
		Description: "The count of attempted claims on expired verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerifyCodeUsed := stats.Int64(MetricPrefix+"/code_used", "The number of attempted claims on already used codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_used_count",
		Measure:     mCodeVerifyCodeUsed,
		Description: "The count of attempted claims on an already used verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerifyInvalid := stats.Int64(MetricPrefix+"/code_invalid", "The number of attempted claims on invalid codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_invalid_count",
		Measure:     mCodeVerifyInvalid,
		Description: "The count of attempted claims on invalid verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerified := stats.Int64(MetricPrefix+"/code_verified", "The number of successfully claimed codes", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/code_verified_count",
		Measure:     mCodeVerified,
		Description: "The count of successfully verified codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCodeVerificationError := stats.Int64(MetricPrefix+"/error", "The number of other errors in code issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/error_count",
		Measure:     mCodeVerificationError,
		Description: "The count of errors issuing verification codes",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Metrics{
		CodeVerifyAttempts:    mCodeVerifyAttempts,
		CodeVerifyExpired:     mCodeVerifyExpired,
		CodeVerifyCodeUsed:    mCodeVerifyCodeUsed,
		CodeVerifyInvalid:     mCodeVerifyInvalid,
		CodeVerified:          mCodeVerified,
		CodeVerificationError: mCodeVerificationError,
	}, nil
}
