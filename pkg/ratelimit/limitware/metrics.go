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

package limitware

import (
	enobs "github.com/google/exposure-notifications-server/pkg/observability"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"github.com/opencensus-integrations/redigo/redis"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/ratelimit/limitware"

var mRequest = stats.Int64(metricPrefix+"/request", "requests seen by middleware", stats.UnitDimensionless)

func init() {
	enobs.CollectViews(append(redis.ObservabilityMetricViews,
		&view.View{
			Name:        metricPrefix + "/request_count",
			Measure:     mRequest,
			Aggregation: view.Count(),
			TagKeys:     append(observability.CommonTagKeys(), enobs.ResultTagKey),
		})...)
}
