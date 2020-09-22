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

// Package associated handles the iOS and Android associated app handler
// protocols. For more discussion of these protocols, please see:
//
// Android:
//   https://developer.android.com/training/app-links/verify-site-associations
// iOS:
//   https://developer.apple.com/documentation/safariservices/supporting_associated_domains
package associated

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	ctxt   context.Context
	db     *database.Database
	cacher cache.Cacher
	h      *render.Renderer
}

var ttl = 30 * time.Minute

func (c *Controller) HandleIos() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.String()

		// Check the cache.
		{
			var data IOSData
			if err := c.cacher.Read(c.ctxt, key, &data); err == nil {
				c.h.RenderJSON(w, http.StatusOK, data)
				return
			} else if err != cache.ErrNotFound {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// Not in cache, go get it.
		data, err := c.getIosData()
		if err != nil {
			c.h.RenderJSON(w, http.StatusOK, api.Error(err))
			return
		}

		// Cache the data.
		if err := c.cacher.Fetch(c.ctxt, key, &data, ttl, func() (interface{}, error) {
			return data, nil
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, data)
	})
}

func (c *Controller) HandleAndroid() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.String()

		// Check the cache
		var data []AndroidData
		if err := c.cacher.Read(c.ctxt, r.URL.String(), &data); err == nil {
			c.h.RenderJSON(w, http.StatusOK, data)
			return
		} else if err != cache.ErrNotFound {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Not in cache, get the data.
		var err error
		data, err = c.getAndroidData()
		if err != nil {
			c.h.RenderJSON(w, http.StatusOK, api.Error(err))
			return
		}

		// And save to the cache.
		if err := c.cacher.Fetch(c.ctxt, key, &data, ttl, func() (interface{}, error) {
			return data, nil
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, data)
	})
}

func New(ctxt context.Context, db *database.Database, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	return &Controller{ctxt: ctxt, db: db, cacher: cacher, h: h}, nil
}
