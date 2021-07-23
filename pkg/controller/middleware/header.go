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

package middleware

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireHeader requires that the request have a certain header present. The
// header just needs to exist - it does not need to have a specific value.
func RequireHeader(header string, h *render.Renderer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v := r.Header.Get(header); v == "" {
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireHeaderValues requires that the request have a certain header present
// and that the value be one of the supplied entries.
func RequireHeaderValues(header string, allowed []string, h *render.Renderer) mux.MiddlewareFunc {
	want := make(map[string]struct{}, len(allowed))
	for _, v := range allowed {
		want[v] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vals := r.Header.Values(header)
			if len(vals) == 0 {
				controller.Unauthorized(w, r, h)
				return
			}

			found := false
			for _, v := range vals {
				if _, ok := want[v]; ok {
					found = true
					break
				}
			}

			if !found {
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
