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
	"strings"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"

	"github.com/gorilla/mux"
)

// googleCloudTraceHeader is the header with trace data.
const googleCloudTraceHeader = "X-Cloud-Trace-Context"

// PopulateTraceID populates the trace ID injected by Google Cloud (if it
// exists).
func PopulateTraceID() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if existing := controller.TraceIDFromContext(ctx); existing == "" {
				if v := r.Header.Get(googleCloudTraceHeader); v != "" {
					parts := strings.Split(v, "/")
					if len(parts) > 0 && len(parts[0]) > 0 {
						ctx = controller.WithTraceID(ctx, project.TrimSpace(parts[0]))
						r = r.Clone(ctx)
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
