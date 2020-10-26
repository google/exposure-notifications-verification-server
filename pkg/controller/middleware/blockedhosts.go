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

// Package middleware defines shared middleware for handlers.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// CheckBlockedHosts can disable requests to particular hostnames.
// It is primarily used to disable the default serving URL in favor of a custom doamin.
func CheckBlockedHosts(ctx context.Context, blockedHosts []string, h *render.Renderer) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.CheckBlockedHosts")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if blockedHosts == nil {
				next.ServeHTTP(w, r)
			}

			for _, host := range blockedHosts {
				if strings.HasSuffix(r.Host, host) {
					logger.Warnf("received request from blocked domain %s", r.Host)
					controller.Unauthorized(w, r, h)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
