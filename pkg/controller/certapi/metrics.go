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
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/certificate"
)

type Metrics struct {
	Attempts          *stats.Int64Measure
	TokenExpired      *stats.Int64Measure
	TokenUsed         *stats.Int64Measure
	TokenInvalid      *stats.Int64Measure
	CertificateIssued *stats.Int64Measure
	CertificateErrors *stats.Int64Measure
}

func registerMetrics() (*Metrics, error) {
	mAttempts := stats.Int64(MetricPrefix+"/attempts", "certificate issue attempts", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/attempt_count",
		Measure:     mAttempts,
		Description: "The count of certificate issue attempts",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mTokenExpired := stats.Int64(MetricPrefix+"/token_expired", "expired tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/token_expired_count",
		Measure:     mTokenExpired,
		Description: "The count of expired tokens on certificate issue",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mTokenUsed := stats.Int64(MetricPrefix+"/token_used", "already used tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/token_used_count",
		Measure:     mTokenUsed,
		Description: "The count of already used tokens on certificate issue",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mTokenInvalid := stats.Int64(MetricPrefix+"/invalid_token", "invalid tokens on certificate issue", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/invalid_token_count",
		Measure:     mTokenInvalid,
		Description: "The count of invalid tokens on certificate issue",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCertificateIssued := stats.Int64(MetricPrefix+"/issue", "certificates issued", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/issue_count",
		Measure:     mCertificateIssued,
		Description: "The count of certificates issued",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}
	mCertificateErrors := stats.Int64(MetricPrefix+"/errors", "certificate issue errors", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/error_count",
		Measure:     mCertificateErrors,
		Description: "The count of certificate issue errors",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Metrics{
		Attempts:          mAttempts,
		TokenExpired:      mTokenExpired,
		TokenUsed:         mTokenUsed,
		TokenInvalid:      mTokenInvalid,
		CertificateIssued: mCertificateIssued,
		CertificateErrors: mCertificateErrors,
	}, nil
}
