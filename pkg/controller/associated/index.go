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
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type Controller struct {
	config           *config.RedirectConfig
	hostnameToRegion map[string]string
	cacher           cache.Cacher
	db               *database.Database
	h                render.Renderer
}

func (c *Controller) getRegion(r *http.Request) string {
	// Get the hostname first
	baseHost := strings.ToLower(r.Host)
	if host, _, err := net.SplitHostPort(baseHost); err == nil {
		baseHost = host
	}

	// return the mapped region code (or default, "", if not found)
	return c.hostnameToRegion[baseHost]
}

func (c *Controller) HandleIos() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("associated.HandleIos")

		region := c.getRegion(r)
		if region == "" {
			c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("request is missing region"))
			return
		}

		// Lookup the realm with the region code.
		realm, err := c.db.FindRealmByRegion(region)
		if err != nil {
			if database.IsNotFound(err) {
				c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("no realm exists for region %q", region))
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		var data *IOSData
		cacheKey := &cache.Key{
			Namespace: "apps:ios:by_region",
			Key:       region,
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &data, c.config.AppCacheTTL, func() (interface{}, error) {
			logger.Debug("fetching new ios data")
			return c.getIosData(realm.ID)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if data == nil {
			c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("no apps are registered"))
			return
		}

		c.h.RenderJSON(w, http.StatusOK, data)
	})
}

func (c *Controller) HandleAndroid() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("associated.HandleAndroid")

		region := c.getRegion(r)
		if region == "" {
			c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("request is missing region"))
			return
		}

		// Lookup the realm with the region code.
		realm, err := c.db.FindRealmByRegion(region)
		if err != nil {
			if database.IsNotFound(err) {
				c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("no realm exists for region %q", region))
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		var data []AndroidData
		cacheKey := &cache.Key{
			Namespace: "apps:android:by_region",
			Key:       region,
		}
		if err := c.cacher.Fetch(ctx, cacheKey, &data, c.config.AppCacheTTL, func() (interface{}, error) {
			logger.Debug("fetching new android data")
			return c.getAndroidData(realm.ID)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if len(data) == 0 {
			c.h.RenderJSON(w, http.StatusNotFound, fmt.Errorf("no apps are registered"))
			return
		}

		c.h.RenderJSON(w, http.StatusOK, data)
	})
}

func New(ctx context.Context, config *config.RedirectConfig, db *database.Database, cacher cache.Cacher, h render.Renderer) (*Controller, error) {
	cfgMap, err := config.HostnameToRegion()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Controller{
		config:           config,
		db:               db,
		cacher:           cacher,
		h:                h,
		hostnameToRegion: cfgMap,
	}, nil
}
