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

package user

import (
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var mUpstreamUserRecreates *stats.Int64Measure

func init() {
	{
		name := observability.MetricRoot + "/user/upstream_user_recreate"
		mUpstreamUserRecreates = stats.Int64(name, "user was re-created in auth provider", stats.UnitDimensionless)
		enobs.CollectViews([]*view.View{
			{
				Name:        name + "_count",
				Measure:     mUpstreamUserRecreates,
				Description: "Count of users that were re-created in the upstream auth provider",
				TagKeys:     observability.CommonTagKeys(),
				Aggregation: view.Count(),
			},
		}...)
	}
}
