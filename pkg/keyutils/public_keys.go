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

// Package keyutils provides helpers for working with ECDSA public keys.
package keyutils

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/pkg/cache"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

type PublicKeyCache struct {
	pubKeyCache *cache.Cache
}

func NewPublicKeyCache(ttl time.Duration) (*PublicKeyCache, error) {
	pubKeyCache, err := cache.New(ttl)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize cache: %w", err)
	}
	return &PublicKeyCache{
		pubKeyCache: pubKeyCache,
	}, nil
}

func (c *PublicKeyCache) GetPublicKey(ctx context.Context, keyID string, kms keys.KeyManager) (crypto.PublicKey, error) {
	// Get the public key for the Token Signing Key.
	keyCache, err := c.pubKeyCache.WriteThruLookup(keyID,
		func() (interface{}, error) {
			signer, err := kms.NewSigner(ctx, keyID)
			if err != nil {
				return nil, err
			}
			return signer.Public(), nil
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get public key for keyId %v: %w", keyID, err)
	}
	publicKey, ok := keyCache.(crypto.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key in wrong format for %v: %w", keyID, err)
	}
	return publicKey, nil
}
