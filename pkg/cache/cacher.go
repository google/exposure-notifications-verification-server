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

// Package cache implements an caches for objects.
package cache

import (
	"context"
	"crypto/hmac"
	"fmt"
	"hash"
	"io"
	"time"
)

var (
	ErrMissingFetchFunc = fmt.Errorf("missing fetch function")
	ErrNotFound         = fmt.Errorf("key not found")
	ErrStopped          = fmt.Errorf("cacher is stopped")
)

// FetchFunc is a function used to Fetch in a cacher.
type FetchFunc func() (interface{}, error)

// Key is a cache key. It has an optional Namespace. Any KeyFunc are applied to
// the Key, but not the Namespace.
type Key struct {
	Namespace string
	Key       string
}

// Compute builds the final value of the key. If f is not nil, it is called on
// the value of Key. If Namespace is not "", the result is prefixed with
// Namespace + ":".
func (k *Key) Compute(f KeyFunc) (string, error) {
	key := k.Key

	if f != nil {
		var err error
		key, err = f(key)
		if err != nil {
			return "", err
		}
	}

	if k.Namespace != "" {
		return k.Namespace + ":" + key, nil
	}
	return key, nil
}

// KeyFunc is a function that mutates the provided cache key before storing it
// in Redis. This can be used to hash or HMAC values to prevent their plaintext
// from appearing in Redis. A good example might be an API key lookup that you
// HMAC before passing along.
//
// The KeyFunc can also be used to add a prefix or namespace to keys in
// multi-tenant systems.
type KeyFunc func(string) (string, error)

// MultiKeyFunc returns a KeyFunc that calls the provided KeyFuncs in order,
// with the previous value passed to the next.
func MultiKeyFunc(fns ...KeyFunc) KeyFunc {
	return func(in string) (string, error) {
		var err error
		for _, fn := range fns {
			if fn == nil {
				continue
			}

			in, err = fn(in)
			if err != nil {
				return "", err
			}
		}

		return in, nil
	}
}

// HashKeyFunc returns a KeyFunc that hashes the provided key before passing it
// to the cacher for storage.
func HashKeyFunc(hasher func() hash.Hash) KeyFunc {
	return func(in string) (string, error) {
		h := hasher()
		n, err := h.Write([]byte(in))
		if err != nil {
			return "", err
		}
		if got, want := n, len(in); got < want {
			return "", fmt.Errorf("only hashed %d of %d bytes", got, want)
		}
		dig := h.Sum(nil)
		return fmt.Sprintf("%x", dig), nil
	}
}

// HMACKeyFunc returns a KeyFunc that HMACs the provided key before passing it
// to the cacher for storage.
func HMACKeyFunc(hasher func() hash.Hash, key []byte) KeyFunc {
	return HashKeyFunc(func() hash.Hash {
		return hmac.New(hasher, key)
	})
}

// Cacher is an interface that defines caching.
type Cacher interface {
	// Closer closes the cache, cleaning up any stale entries. A closed cache is
	// no longer valid and attempts to call methods should return an error or
	// (less preferred) panic.
	io.Closer

	// Fetch retrieves the named item from the cache. If the item does not exist,
	// it calls FetchFunc to create the item. If FetchFunc returns an error, the
	// error is bubbled up the stack and no value is cached. If FetchFunc
	// succeeds, the value is cached for the provided TTL.
	Fetch(context.Context, *Key, interface{}, time.Duration, FetchFunc) error

	// Read gets an item from the cache and reads it into the provided interface.
	// If it does not exist, it returns ErrNotFound.
	Read(context.Context, *Key, interface{}) error

	// Write adds an item to the cache, overwriting if it already exists, caching
	// for TTL. It returns any errors that occur on writing.
	Write(context.Context, *Key, interface{}, time.Duration) error

	// Delete removes an item from the cache, returning any errors that occur.
	Delete(context.Context, *Key) error

	// DeletePrefix removes all items from the cache that begin with the given
	// value.
	DeletePrefix(context.Context, string) error
}
