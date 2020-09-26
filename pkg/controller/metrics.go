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
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const contextKeyMetrics = contextKey("controllerMetricsKey")

var MetricPrefix = observability.MetricRoot + "/server/shared"

type Metrics struct {
	FirebaseRecreates *stats.Int64Measure
}

func MetricsFromContext(ctx context.Context) (context.Context, *Metrics, error) {
	v := ctx.Value(contextKeyMetrics)
	if v == nil {
		return registerMetrics(ctx)
	}

	if m, ok := v.(*Metrics); ok {
		return ctx, m, nil
	}
	return registerMetrics(ctx)
}

func registerMetrics(ctx context.Context) (context.Context, *Metrics, error) {
	mFirebaseRecreates := stats.Int64(MetricPrefix+"/fb_recreate", "recreation of firebase users", stats.UnitDimensionless)
	if err := view.Register(&view.View{
		Name:        MetricPrefix + "/fb_recreate_count",
		Measure:     mFirebaseRecreates,
		Description: "The count of firebase account recreations",
		TagKeys:     []tag.Key{observability.RealmTagKey},
		Aggregation: view.Count(),
	}); err != nil {
		return ctx, nil, fmt.Errorf("stat view registration failure: %w", err)
	}

	metrics := &Metrics{FirebaseRecreates: mFirebaseRecreates}
	return context.WithValue(ctx, contextKeyMetrics, metrics), metrics, nil
}
