// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/database"

var mAuditEntryCreated = stats.Int64(metricPrefix+"/audit_entry_created", "The number of times an audit entry was created", stats.UnitDimensionless)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/audit_entry_created_count",
			Measure:     mAuditEntryCreated,
			Description: "The count of the number of audit entries created",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
	}...)
}
