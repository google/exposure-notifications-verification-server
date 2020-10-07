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
	"go.opencensus.io/tag"
)

const metricPrefix = observability.MetricRoot + "/server/login"

var (
	MFirebaseRecreates = stats.Int64(metricPrefix+"/fb_recreates", "firebase user recreates", stats.UnitDimensionless)
)

var (
	// tagBlame indicating Who to blame for the API request failure.
	// NONE: no failure
	// CLIENT: the client is at fault (e.g. invalid request)
	// SERVER: the server is at fault
	// EXTERNAL: some third party is at fault
	// UNKNOWN: for everything else
	tagBlame = tag.MustNewKey("blame")

	// tagResult contains a free format text describing the result of the
	// request. Preferrably ALL CAPS WITH UNDERSCORE.
	// OK indicating a successful request.
	// You can losely base this string on
	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// but feel free to use any text as long as it's easy to filter.
	tagResult = tag.MustNewKey("result")
)

var (
	// BlameNone indicate no API failure
	BlameNone = tag.Upsert(tagBlame, "NONE")

	// BlameClient indicate the client is at fault (e.g. invalid request)
	BlameClient = tag.Upsert(tagBlame, "CLIENT")

	// BlameServer indicate the server is at fault
	BlameServer = tag.Upsert(tagBlame, "SERVER")

	// BlameExternal indicate some third party is at fault
	BlameExternal = tag.Upsert(tagBlame, "EXTERNAL")

	// BlameUnknown can be used for everything else
	BlameUnknown = tag.Upsert(tagBlame, "UNKOWN")
)

// APIResultOK add a tag indicating the API call is a success.
func APIResultOK() tag.Mutator {
	return tag.Upsert(tagResult, "OK")
}

// APIResultError add a tag with the given string as the result.
func APIResultError(result string) tag.Mutator {
	return tag.Upsert(tagResult, result)
}

// APITagKeys return a slice of tag.Key with common tag keys + additional API
// specific tag keys.
func APITagKeys() []tag.Key {
	return append(observability.CommonTagKeys(), tagBlame, tagResult)
}

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
