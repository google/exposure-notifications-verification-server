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

package middleware

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"go.uber.org/zap"
)

type TemplateMiddleware struct {
	config *config.ServerConfig
	h      *render.Renderer
	logger *zap.SugaredLogger
}

func PopulateTemplateVariables(ctx context.Context, config *config.ServerConfig, h *render.Renderer) *TemplateMiddleware {
	logger := logging.FromContext(ctx)

	return &TemplateMiddleware{
		config: config,
		h:      h,
		logger: logger,
	}
}

func (h *TemplateMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		m := controller.TemplateMapFromContext(ctx)
		if m == nil {
			m = make(controller.TemplateMap)
		}

		if _, ok := m["server"]; !ok {
			m["server"] = h.config.ServerName
		}
		if _, ok := m["title"]; !ok {
			m["title"] = h.config.ServerName
		}

		r = r.WithContext(controller.WithTemplateMap(ctx, m))
		next.ServeHTTP(w, r)
	})
}
