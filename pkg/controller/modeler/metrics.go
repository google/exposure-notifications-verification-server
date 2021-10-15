// Copyright 2021 the Exposure Notifications Verification Server authors
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

package modeler

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/modeler"

var (
	mSuccess = stats.Int64(metricPrefix+"/success", "successful execution", stats.UnitDimensionless)

	mCodesClaimedRatioAnomaly = stats.Int64(metricPrefix+"/codes_claimed_ratio_anomaly", "an anomaly occurred with the ratio of codes issued to codes claimed", stats.UnitDimensionless)
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/success",
			Description: "Number of successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/codes_claimed_ratio_anomaly",
			Description: "Number of times an anomaly occurred with the ratio of codes issued to codes claimed",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mCodesClaimedRatioAnomaly,
			Aggregation: view.Count(),
		},
	}...)
}
