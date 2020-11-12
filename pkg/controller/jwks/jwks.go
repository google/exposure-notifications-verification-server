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

// Package jwks handles returning JSON encoded information about the
// server's encryptionn keys. See https://tools.ietf.org/html/rfc75170
// for information about the server.
package jwks

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/rakutentech/jwk-go/jwk"
)

// Error codes
var errBadRealm = errors.New("bad realm")

// Controller holds all the pieces necessary to show the jwks encoded keys.
type Controller struct {
	h        *render.Renderer
	db       *database.Database
	keyCache *keyutils.PublicKeyCache
	cacher   cache.Cacher
}

// HandleIndex returns an http.Handler that handles jwks GET requests.
func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// key is the key in the cacher where the values for this JWK are cached.
		key := &cache.Key{
			Namespace: "jwks",
			Key:       r.URL.String(),
		}

		// See if there's a cached value. Note we cannot use Fetch here because our
		// fetch function also depends on the cacher to lookup pubic keys and
		// results in a deadlock.
		var encoded []*jwk.JWK
		if err := c.cacher.Read(ctx, key, &encoded); err == nil {
			c.h.RenderJSON(w, http.StatusOK, encoded)
			return
		} else if err != cache.ErrNotFound {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// If we got this far, it means there was no cached value, so do a full
		// read.
		encoded, err := func() ([]*jwk.JWK, error) {
			// Grab the URL path components  we need.
			realmID := mux.Vars(r)["realm_id"]

			// Find the realm, and the key abstractions from the DB.
			realm, err := c.getRealm(realmID)
			if err != nil {
				return nil, err
			}

			var keys []*database.SigningKey
			keys, err = realm.ListSigningKeys(c.db)
			if err != nil {
				return nil, err
			}

			encoded := make([]*jwk.JWK, len(keys))
			for i, key := range keys {
				pk, err := c.keyCache.GetPublicKey(ctx, key.KeyID, c.db.KeyManager())
				if err != nil {
					return nil, err
				}

				// Encode it, and sent it off.
				spec := jwk.NewSpec(pk)
				spec.KeyID = key.GetKID()
				encoded[i], err = spec.ToJWK()
				if err != nil {
					return nil, err
				}
			}
			return encoded, nil
		}()
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// It's possible there were concurrent requests and someone already has the
		// cache - now that we have the value, we can avoid the deadline and do a
		// fetch. If there's already a cached value, our value will be discarded.
		// Otherwise, it will be overwritten and saved in the cache.
		ttl := 5 * time.Minute
		if err := c.cacher.Fetch(ctx, key, &encoded, ttl, func() (interface{}, error) {
			return encoded, nil
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Get the keys.
		c.h.RenderJSON(w, http.StatusOK, encoded)
	})
}

// getRealm finds realm given ID.
func (c *Controller) getRealm(realmStr string) (*database.Realm, error) {
	realmID, err := strconv.Atoi(realmStr)
	if err != nil {
		return nil, errBadRealm
	}
	var realm *database.Realm
	realm, err = c.db.FindRealm(uint(realmID))
	if err != nil {
		return nil, errBadRealm
	}
	return realm, nil
}

// New creates a new jwks *Controller, and returns it.
func New(ctx context.Context, db *database.Database, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	kc, err := keyutils.NewPublicKeyCache(ctx, cacher, time.Minute)
	if err != nil {
		return nil, err
	}

	return &Controller{
		h:        h,
		db:       db,
		keyCache: kc,
		cacher:   cacher,
	}, nil
}
