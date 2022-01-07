// Copyright 2022 the Exposure Notifications Verification Server authors
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

package emailer

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/emailer"

var (
	mAnomaliesSuccess = stats.Int64(metricPrefix+"/anomalies_success", "successful anomalies emails", stats.UnitDimensionless)
	mSMSErrorsSuccess = stats.Int64(metricPrefix+"/sms_errors_success", "successful SMS errors emails", stats.UnitDimensionless)
)

func init() {
	enobs.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/anomalies/success",
			Description: "Number of anomalies email successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mAnomaliesSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/sms_errors/success",
			Description: "Number of SMS errors email successes",
			TagKeys:     observability.CommonTagKeys(),
			Measure:     mSMSErrorsSuccess,
			Aggregation: view.Count(),
		},
	}...)
}
