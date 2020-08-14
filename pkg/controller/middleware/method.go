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
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/gorilla/mux"
)

// MutateMethod looks for HTML form values that define the "real" HTTP method
// and then forward that along to the router. This must be a very early
// middleware.
func MutateMethod(ctx context.Context) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.MutateMethod")
	formKeyMethod := "_method"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := strings.ToUpper(r.FormValue(formKeyMethod))
			if method != "" {
				logger.Debugw("overriding method", "old", r.Method, "new", method)
				r.Method = method
			}

			next.ServeHTTP(w, r)
		})
	}
}
