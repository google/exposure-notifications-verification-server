// Copyright 2021 Google LLC
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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/mux"
)

// RequireHostHeader requires that the request's host header is one of the allowed values.
func RequireHostHeader(allowed []string, h *render.Renderer, stripPort bool) mux.MiddlewareFunc {
	want := make(map[string]struct{}, len(allowed))
	for _, v := range allowed {
		want[strings.ToLower(v)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := strings.ToLower(r.Host)

			if stripPort {
				if i := strings.Index(host, ":"); i > 0 {
					host = host[0:i]
				}
			}

			if _, ok := want[host]; !ok {
				controller.Unauthorized(w, r, h)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
