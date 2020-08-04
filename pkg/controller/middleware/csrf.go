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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
)

// ConfigureCSRF injects the CSRF handling and populates the global template map
// with the csrfToken and csrfTemplate.
func ConfigureCSRF(ctx context.Context, authKey []byte, options ...csrf.Option) mux.MiddlewareFunc {
	logger := logging.FromContext(ctx).Named("middleware.ConfigureCSRF")

	protect := csrf.Protect(authKey, options...)

	return func(next http.Handler) http.Handler {
		return protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Save csrf configuration on the template map.
			m := controller.TemplateMapFromContext(ctx)
			if _, ok := m["csrfField"]; !ok {
				m["csrfField"] = csrf.TemplateField(r)
			}
			if _, ok := m["csrfToken"]; !ok {
				m["csrfToken"] = csrf.Token(r)
			}

			// Save the template map on the context.
			ctx = controller.WithTemplateMap(ctx, m)
			*r = *r.WithContext(ctx)

			logger.Debugw("done")
			next.ServeHTTP(w, r)
		}))
	}
}
