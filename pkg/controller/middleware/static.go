// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"time"

	"github.com/gorilla/mux"
)

// ConfigureStaticAssets configures headers for static assets.
func ConfigureStaticAssets(devMode bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do not cache assets in dev mode.
			if devMode {
				w.Header().Set("Cache-Control", "private, no-cache, max-age=0")
				w.Header().Set("Expires", time.Now().Add(-30*time.Minute).Format(http.TimeFormat))
				w.Header().Set("Vary", "Accept-Encoding")
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Header().Set("Expires", time.Now().AddDate(1, 0, 0).Format(http.TimeFormat))
			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(w, r)
		})
	}
}
