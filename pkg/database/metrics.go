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

package database

import (
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	MetricPrefix = observability.MetricRoot + "/database"
)

type Metrics struct {
	AuditEntryCreated *stats.Int64Measure
}

func registerMetrics() (*Metrics, error) {
	mAuditEntryCreated := stats.Int64(MetricPrefix+"/audit_entry_created", "The number of times an audit entry was created", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/audit_entry_created_count",
		Measure:     mAuditEntryCreated,
		Description: "The count of the number of audit entries created",
		TagKeys:     observability.CommonTagKeys(),
		Aggregation: view.Count(),
	}); err != nil {
		return nil, fmt.Errorf("failed to register audit_entry_created: %w", err)
	}

	return &Metrics{
		AuditEntryCreated: mAuditEntryCreated,
	}, nil
}
