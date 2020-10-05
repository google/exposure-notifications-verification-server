// Package observability provides tools for working with open census.
package observability

import (
	"context"
	"os"
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

	KnativeServiceTagKey       = tag.MustNewKey("k_service")
	KnativeRevisionTagKey      = tag.MustNewKey("k_revision")
	KnativeConfigurationTagKey = tag.MustNewKey("k_configuration")

	RealmTagKey = tag.MustNewKey("realm")

	knativeService       = os.Getenv("K_SERVICE")
	knativeRevision      = os.Getenv("K_REVISION")
	knativeConfiguration = os.Getenv("K_CONFIGURATION")
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

// WithBuildInfo creates a new context with the build and revision info attached
// to the observability context.
func WithBuildInfo(octx context.Context) context.Context {
	tags := make([]tag.Mutator, 0, 5)
	tags = append(tags, tag.Upsert(BuildIDTagKey, buildinfo.BuildID))
	tags = append(tags, tag.Upsert(BuildTagTagKey, buildinfo.BuildTag))

	if knativeService != "" {
		tags = append(tags, tag.Upsert(KnativeServiceTagKey, knativeService))
	}

	if knativeRevision != "" {
		tags = append(tags, tag.Upsert(KnativeRevisionTagKey, knativeRevision))
	}

	if knativeConfiguration != "" {
		tags = append(tags, tag.Upsert(KnativeConfigurationTagKey, knativeConfiguration))
	}

	ctx, err := tag.New(octx, tags...)
	if err != nil {
		logger := logging.FromContext(octx).Named("observability.WithBuildInfo")
		logger.Errorw("failed to upsert buildinfo on observability context", "error", err)
		return octx
	}

	return ctx
}
