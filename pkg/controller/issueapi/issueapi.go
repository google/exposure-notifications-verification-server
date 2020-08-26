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

// Package issueapi implements the API handler for taking a code request, assigning
// an OTP, saving it to the database and returning the result.
// This is invoked over AJAX from the Web frontend.
package issueapi

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"go.uber.org/zap"
)

var (
	MetricPrefix = observability.MetricRoot + "/api/issue"
)

type Controller struct {
	config config.IssueAPIConfig
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger

	validTestType map[string]struct{}

	mCodesIssued     *stats.Int64Measure
	mCodeIssueErrors *stats.Int64Measure
	mSMSSent         *stats.Int64Measure
	mSMSSendErrors   *stats.Int64Measure
}

// New creates a new IssueAPI controller.
func New(ctx context.Context, config config.IssueAPIConfig, db *database.Database, h *render.Renderer) (*Controller, error) {
	// Set up metrics.
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
	mCodeIssueErrors := stats.Int64(MetricPrefix+"code_issue_error", "The number of failed code issues", stats.UnitDimensionless)
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
	mSMSSendErrors := stats.Int64(MetricPrefix+"sms_send_error", "The number of failed SMS sends", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/sms_send_error_count",
		Measure:     mSMSSendErrors,
		Description: "The count of the number of a code issue failed due to SMS send failure",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	return &Controller{
		config: config,
		db:     db,
		h:      h,
		logger: logging.FromContext(ctx),
		validTestType: map[string]struct{}{
			api.TestTypeConfirmed: {},
			api.TestTypeLikely:    {},
			api.TestTypeNegative:  {},
		},
		mCodesIssued:     mCodesIssued,
		mCodeIssueErrors: mCodeIssueErrors,
		mSMSSent:         mSMSSent,
		mSMSSendErrors:   mSMSSendErrors,
	}, nil
}
