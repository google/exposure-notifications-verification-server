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
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"
)

type Controller struct {
	config           *config.RedirectConfig
	hostnameToRegion map[string]string
	cacher           cache.Cacher
	db               *database.Database
	h                *render.Renderer
	logger           *zap.SugaredLogger
}

func (c *Controller) getRegion(r *http.Request) string {
	// Get the hostname first
	host := strings.ToLower(r.Host)
	if i := strings.Index(host, ":"); i > 0 {
		host = host[0:i]
	}

	// return the mapped region code (or default, "", if not found)
	return c.hostnameToRegion[host]
}

func (c *Controller) HandleIos() http.Handler {
	notFound := api.Errorf("not found")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		region := c.getRegion(r)
		if region == "" {
			c.h.RenderJSON(w, http.StatusNotFound, notFound)
			return
		}

		cacheKey := &cache.Key{
			Namespace: "apps:ios:by_region",
			Key:       region,
		}
		var iosData *IOSData
		if err := c.cacher.Fetch(ctx, cacheKey, &iosData, c.config.AppCacheTTL, func() (interface{}, error) {
			c.logger.Debug("fetching new ios data")
			return c.getIosData(region)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if iosData == nil {
			c.h.RenderJSON(w, http.StatusNotFound, notFound)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, iosData)
	})
}

func (c *Controller) HandleAndroid() http.Handler {
	notFound := api.Errorf("not found")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		region := c.getRegion(r)
		if region == "" {
			c.h.RenderJSON(w, http.StatusNotFound, notFound)
			return
		}

		cacheKey := &cache.Key{
			Namespace: "apps:android:by_region",
			Key:       region,
		}
		var androidData []AndroidData
		if err := c.cacher.Fetch(ctx, cacheKey, &androidData, c.config.AppCacheTTL, func() (interface{}, error) {
			c.logger.Debug("fetching new android data")
			return c.getAndroidData(region)
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		if len(androidData) == 0 {
			c.h.RenderJSON(w, http.StatusNotFound, notFound)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, androidData)
	})
}

func New(ctx context.Context, config *config.RedirectConfig, db *database.Database, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	logger := logging.FromContext(ctx).Named("associated")

	cfgMap, err := config.HostnameToRegion()
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Controller{
		config:           config,
		db:               db,
		cacher:           cacher,
		h:                h,
		logger:           logger,
		hostnameToRegion: cfgMap,
	}, nil
}
