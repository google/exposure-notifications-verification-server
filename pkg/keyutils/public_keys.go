// Copyright 2020 the Exposure Notifications Verification Server authors
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

// Package keyutils provides helpers for working with ECDSA public keys.
package keyutils

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
)

type PublicKeyCache struct {
	cacher cache.Cacher
	ttl    time.Duration
}

// NewPublicKeyCache creates a new public key cache from the given parameters.
func NewPublicKeyCache(ctx context.Context, cacher cache.Cacher, ttl time.Duration) (*PublicKeyCache, error) {
	return &PublicKeyCache{
		cacher: cacher,
		ttl:    ttl,
	}, nil
}

// GetPublicKey returns the public key for the provided ID.
func (c *PublicKeyCache) GetPublicKey(ctx context.Context, id string, kms keys.KeyManager) (crypto.PublicKey, error) {
	cacheKey := &cache.Key{
		Namespace: "public_keys",
		Key:       id,
	}

	var b []byte
	if err := c.cacher.Fetch(ctx, cacheKey, &b, c.ttl, func() (interface{}, error) {
		signer, err := kms.NewSigner(ctx, id)
		if err != nil {
			return nil, err
		}
		return x509.MarshalPKIXPublicKey(signer.Public())
	}); err != nil {
		return nil, fmt.Errorf("failed to fetch public key for %s: %w", id, err)
	}

	raw, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	switch pub := raw.(type) {
	case *rsa.PublicKey:
		return pub, nil
	case *ecdsa.PublicKey:
		return pub, nil
	case ed25519.PublicKey:
		return pub, nil
	default:
		return fmt.Errorf("unknown public key type: %T", pub), nil
	}
}
