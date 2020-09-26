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

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"
)

type Controller struct {
	cacher cache.Cacher
	db     *database.Database
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// cacheTTL is the amount of time for which to cache these values.
const cacheTTL = 30 * time.Minute

func (c *Controller) HandleIos() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		cacheKey := r.URL.String()

		var iosData IOSData
		if err := c.cacher.Fetch(ctx, cacheKey, &iosData, cacheTTL, func() (interface{}, error) {
			c.logger.Debug("fetching new ios data")
			return c.getIosData()
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, iosData)
	})
}

func (c *Controller) HandleAndroid() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		cacheKey := r.URL.String()

		var androidData AndroidData
		if err := c.cacher.Fetch(ctx, cacheKey, &androidData, cacheTTL, func() (interface{}, error) {
			c.logger.Debug("fetching new android data")
			return c.getAndroidData()
		}); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		c.h.RenderJSON(w, http.StatusOK, androidData)
	})
}

func New(ctx context.Context, db *database.Database, cacher cache.Cacher, h *render.Renderer) (*Controller, error) {
	logger := logging.FromContext(ctx).Named("associated")

	return &Controller{
		db:     db,
		cacher: cacher,
		h:      h,
		logger: logger,
	}, nil
}
