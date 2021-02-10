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

// Package observability provides tools for working with open census.
package observability

import (
	"context"
	"strconv"

	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/tag"
)

const (
	MetricRoot = "en-verification-server"
)

var (
	RealmTagKey = tag.MustNewKey("realm")
)

// CommonTagKeys returns the slice of common tag keys that should used in all
// views.
func CommonTagKeys() []tag.Key {
	return []tag.Key{
		enobs.BuildIDTagKey,
		enobs.BuildTagTagKey,
		RealmTagKey,
	}
}

// APITagKeys return a slice of tag.Key with common tag keys + additional API
// specific tag keys.
func APITagKeys() []tag.Key {
	return append(CommonTagKeys(), enobs.BlameTagKey, enobs.ResultTagKey)
}

// WithRealmID creates a new context with the realm id attached to the
// observability context.
func WithRealmID(octx context.Context, realmID uint64) context.Context {
	realmIDStr := strconv.FormatUint(realmID, 10)
	ctx, err := tag.New(octx, tag.Upsert(RealmTagKey, realmIDStr))
	if err != nil {
		logger := logging.FromContext(octx).Named("observability.WithRealmID")
		logger.Errorw("failed to upsert realm on observability context",
			"error", err,
			"realm_id", realmIDStr)
		return octx
	}
	return ctx
}
