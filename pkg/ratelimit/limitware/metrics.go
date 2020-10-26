package limitware

import (
	enobservability "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/opencensus-integrations/redigo/redis"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/ratelimit/limitware"

var (
	mRequest = stats.Int64(metricPrefix+"/request", "requests seen by middleware", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews(append(redis.ObservabilityMetricViews,
		&view.View{
			Name:        metricPrefix + "/request_count",
			Measure:     mRequest,
			Aggregation: view.Count(),
			TagKeys:     append(observability.CommonTagKeys(), observability.ResultTagKey),
		})...)
}
