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
	"net/http"

	"github.com/google/exposure-notifications-verification-server/internal/buildinfo"
	"github.com/gorilla/mux"
)

const (
	HeaderDebug         = "x-debug"
	HeaderDebugBuildID  = "x-build-id"
	HeaderDebugBuildTag = "x-build-tag"
)

// ProcessDebug adds additional debugging information to the response if the
// request included the "X-Debug" header with any value.
func ProcessDebug() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(HeaderDebug) != "" {
				w.Header().Set(HeaderDebugBuildID, buildinfo.BuildID)
				w.Header().Set(HeaderDebugBuildTag, buildinfo.BuildTag)
			}

			next.ServeHTTP(w, r)
		})
	}
}
