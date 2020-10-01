// Package observability provides tools for working with open census.
package observability

import (
	"context"
	"strconv"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/buildinfo"
	"go.opencensus.io/tag"
)

const (
	MetricRoot = "en-verification-server"
)

var (
	BuildIDTagKey  = tag.MustNewKey("build_id")
	BuildTagTagKey = tag.MustNewKey("build_tag")
	RealmTagKey    = tag.MustNewKey("realm")
)

// CommonTagKeys returns the slice of common tag keys that should used in all
// views.
func CommonTagKeys() []tag.Key {
	return []tag.Key{
		BuildIDTagKey,
		BuildTagTagKey,
		RealmTagKey,
	}
}

// WithRealmID creates a new context with the realm id attached to the
// observability context.
func WithRealmID(octx context.Context, realmID uint) context.Context {
	realmIDStr := strconv.FormatUint(uint64(realmID), 10)
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

// WithBuildInfo creates a new context with the build info attached to the
// observability context.
func WithBuildInfo(octx context.Context) context.Context {
	ctx, err := tag.New(octx,
		tag.Upsert(BuildIDTagKey, buildinfo.BuildID),
		tag.Upsert(BuildTagTagKey, buildinfo.BuildTag))
	if err != nil {
		logger := logging.FromContext(octx).Named("observability.WithBuildInfo")
		logger.Errorw("failed to upsert buildinfo on observability context", "error", err)
		return octx
	}
	return ctx
}
