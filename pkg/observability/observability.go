// Package observability provides tools for working with open census.
package observability

import (
	"log"

	"go.opencensus.io/tag"
)

const (
	MetricRoot = "en-verification-server"
)

var (
	RealmTagKey tag.Key
)

func init() {
	var err error
	RealmTagKey, err = tag.NewKey("realm")
	if err != nil {
		log.Fatalf("invalid observability tag: %v", err)
	}
}
