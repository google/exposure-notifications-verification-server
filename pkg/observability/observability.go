// Package observability provides tools for working with open census.
package observability

import "go.opencensus.io/tag"

const (
	MetricRoot = "en-verification-server"
)

var (
	RealmTagKey, _ = tag.NewKey("realm")
)
