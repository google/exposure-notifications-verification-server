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

	"github.com/gorilla/mux"
)

// QueryHeaderInjection is for development and should not be installed in production flows.
// This middleware will take query params from a get request and copy them to
func QueryHeaderInjection(header, queryParam string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If this isn't a get request, do not attempt injection.
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			// If the header already exists, skip.
			if cur := strings.TrimSpace(r.Header.Get(header)); cur != "" {
				next.ServeHTTP(w, r)
				return
			}

			// If there is an query param, inject it.
			if fromQuery := r.URL.Query().Get(queryParam); fromQuery != "" {
				r.Header.Set(header, fromQuery)
			}

			next.ServeHTTP(w, r)
		})
	}
}
