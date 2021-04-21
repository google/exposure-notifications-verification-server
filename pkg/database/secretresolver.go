// Copyright 2021 Google LLC
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

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

type SecretResolver struct {
	// referencesCache maps database values to their references. This is a small
	// TTL-based cache that exists to limit load on the database. It maps a secret
	// type to the result from the database query.
	//
	//   database(cookie_key) -> projects/p/secrets/s/versions/1
	//
	referencesCache *cache.Cache

	// valuesCache maps the references to their actual secret values. Since secret
	// versions are immutable, this is a long-running cache.
	//
	//   projects/p/secrets/s/versions/1 -> abcd1234
	//
	valuesCache *cache.Cache
}

// NewSecretResolver makes a new secret resolver using the provided database and
// secret manager instance.
func NewSecretResolver() *SecretResolver {
	// These only return an error if the duration is < 0.
	referencesCache, _ := cache.New(60 * time.Second)
	valuesCache, _ := cache.New(120 * time.Minute)

	return &SecretResolver{
		referencesCache: referencesCache,
		valuesCache:     valuesCache,
	}
}

// Resolve resolves all secrets for the provided database secret type to their
// upstream secret manager values. The mappings of database values to references
// are cached for a short duration, and the mappings of references to secret
// values are cached for a very long time (since secret versions are immutable).
func (r *SecretResolver) Resolve(ctx context.Context, db *Database, sm secrets.SecretManager, typ SecretType) ([][]byte, error) {
	references, err := r.ResolveReferences(db, typ)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references for %s: %w", typ, err)
	}

	results := make([][]byte, len(references))
	for i, reference := range references {
		value, err := r.ResolveValue(ctx, sm, reference)
		if err != nil {
			return nil, err
		}
		results[i] = value
	}
	return results, nil
}

// ResolveReferences resolves the database references for the given secret type,
// handling caching where appropriate.
func (r *SecretResolver) ResolveReferences(db *Database, typ SecretType) ([]string, error) {
	iface, err := r.referencesCache.WriteThruLookup(string(typ), func() (interface{}, error) {
		records, err := db.ListSecretsForType(typ)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets for %s: %w", typ, err)
		}

		references := make([]string, len(records))
		for i, record := range records {
			references[i] = record.Reference
		}
		return references, nil
	})
	if err != nil {
		return nil, err
	}

	list, ok := iface.([]string)
	if !ok {
		return nil, fmt.Errorf("result for %s is not []string: %T", typ, iface)
	}
	return list, nil
}

// ResolveValue resolves a single secret, taking caching into account.
func (r *SecretResolver) ResolveValue(ctx context.Context, sm secrets.SecretManager, ref string) ([]byte, error) {
	iface, err := r.valuesCache.WriteThruLookup(ref, func() (interface{}, error) {
		result, err := sm.GetSecretValue(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret value %s: %w", ref, err)
		}
		return []byte(result), nil
	})
	if err != nil {
		return nil, err
	}

	typ, ok := iface.([]byte)
	if !ok {
		return nil, fmt.Errorf("result for %s is not []byte: %T", ref, iface)
	}
	return typ, nil
}

// ClearCaches purges all cached data from the references and values cache.
func (r *SecretResolver) ClearCaches() {
	r.referencesCache.Clear()
	r.valuesCache.Clear()
}
