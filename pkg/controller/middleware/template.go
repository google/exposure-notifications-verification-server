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
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"

	"github.com/gorilla/mux"
)

// PopulateTemplateVariables populates the template variables with common
// information and bootstraps the map for more values to be set by other
// middlewares.
func PopulateTemplateVariables(config *config.ServerConfig) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			m := controller.TemplateMapFromContext(ctx)
			m["server"] = config.ServerName
			m["title"] = config.ServerName
			m["buildID"] = buildinfo.BuildID
			m["buildTag"] = buildinfo.BuildTag
			m["devMode"] = config.DevMode
			m["systemNotice"] = config.ParsedSystemNotice()

			// Add in any feature flags.
			m = config.Features.AddToTemplate(m)

			// Save the template map on the context.
			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
