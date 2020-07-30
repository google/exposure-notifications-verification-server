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

// Package index defines the controller for the index/landing page.
package index

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware/html"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"

	"github.com/gorilla/csrf"
)

type Controller struct {
	config *config.ServerConfig
	h      *render.Renderer
	logger *zap.SugaredLogger
}

// New creates a new index controller. The index controller is thread-safe.
func New(ctx context.Context, config *config.ServerConfig, h *render.Renderer) *Controller {
	logger := logging.FromContext(ctx)

	return &Controller{
		config: config,
		h:      h,
		logger: logger,
	}
}

func (c *Controller) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := csrf.Token(r)

		m := html.GetTemplateMap(r)
		m["firebase"] = c.config.Firebase
		m["csrftoken"] = token

		w.Header().Add("X-CSRF-Token", token)
		c.h.RenderHTML(w, "index", m)
	})
}
