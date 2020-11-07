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

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireHeader requires that the request have a certain header present. The
// header just needs to exist - it does not need to have a specific value.
func RequireHeader(h *render.Renderer, header string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logging.FromContext(ctx).Named("middleware.RequireHeader")

			if v := r.Header.Get(header); v == "" {
				logger.Debugw("missing required header", "header", header)
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireHeaderValues requires that the request have a certain header present
// and that the value be one of the supplied entries.
func RequireHeaderValues(ctx context.Context, h *render.Renderer, header string, allowed []string) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.RequireHeaderValue")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vals := r.Header.Values(header)
			if len(vals) == 0 {
				logger.Debugw("missing required header", "header", header)
				controller.Unauthorized(w, r, h)
				return
			}

			found := false
		LOOP:
			for _, v := range vals {
				for _, a := range allowed {
					if a == v {
						found = true
						break LOOP
					}
				}
			}

			if !found {
				logger.Debugw("header does not have allowed values",
					"header", header,
					"values", vals,
					"allowed", allowed)
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
