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

package controller

import (
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	enobservability "github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = observability.MetricRoot + "/server/login"

var (
	MFirebaseRecreates = stats.Int64(metricPrefix+"/fb_recreates", "firebase user recreates", stats.UnitDimensionless)
)

func init() {
	enobservability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/fb_recreate_count",
			Measure:     MFirebaseRecreates,
			Description: "The count of firebase user recreations",
			TagKeys:     observability.CommonTagKeys(),
			Aggregation: view.Count(),
		},
	}...)
}
