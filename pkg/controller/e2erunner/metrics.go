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

package e2erunner

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/e2e"

var (
	mDefaultSuccess    = stats.Int64(metricPrefix+"/default/success", "successful default execution", stats.UnitDimensionless)
	mRevisionSuccess   = stats.Int64(metricPrefix+"/revision/success", "successful revision execution", stats.UnitDimensionless)
	mRedirectSuccess   = stats.Int64(metricPrefix+"/redirect/success", "successful redirect execution", stats.UnitDimensionless)
	mUserReportSuccess = stats.Int64(metricPrefix+"/user-report/success", "successful user-report execution", stats.UnitDimensionless)
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/default/success",
			Description: "Number of default successes",
			Measure:     mDefaultSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/revision/success",
			Description: "Number of revision successes",
			Measure:     mRevisionSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/redirect/success",
			Description: "Number of redirect successes",
			Measure:     mRedirectSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/user-report/success",
			Description: "Number of user-report successes",
			Measure:     mUserReportSuccess,
			Aggregation: view.Count(),
		},
	}...)
}
