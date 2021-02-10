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
	"html/template"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
)

// ConfigureCSRF injects the CSRF handling and populates the global template map
// with the csrfToken and csrfTemplate.
func ConfigureCSRF(config *config.ServerConfig, h *render.Renderer) mux.MiddlewareFunc {
	protect := csrf.Protect(config.CSRFAuthKey,
		csrf.Secure(!config.DevMode),
		csrf.ErrorHandler(handleCSRFError(h)),
	)

	return func(next http.Handler) http.Handler {
		return protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Save csrf configuration on the template map.
			m := controller.TemplateMapFromContext(ctx)
			m["csrfField"] = csrf.TemplateField(r)
			m["csrfToken"] = csrf.Token(r)
			m["csrfMeta"] = template.HTML(
				`<meta name="csrf-token" content="` + csrf.Token(r) + `">`)

			// Save the template map on the context.
			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		}))
	}
}

// handleCSRFError is an http.HandlerFunc that can be installed in the gorilla
// csrf protect middleware. It will respond w/ a JSON object containing error:
// on API requests and a signout redirect to other requests.
func handleCSRFError(h *render.Renderer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("middleware.handleCSRFError")

		reason := csrf.FailureReason(r)
		logger.Debugw("invalid csrf", "reason", reason)

		controller.Unauthorized(w, r, h)
		return
	})
}
