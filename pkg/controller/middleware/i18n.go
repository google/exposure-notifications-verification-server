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

	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"

	"github.com/gorilla/mux"
)

const (
	HeaderAcceptLanguage = "Accept-Language"
	QueryKeyLanguage     = "lang"
)

// ProcessLocale extracts the locale from the various possible locations and
// sets the template translator to the correct language.
//
// This must be called after the template map has been created.
func ProcessLocale(locales *i18n.LocaleMap) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			param := r.URL.Query().Get(QueryKeyLanguage)
			header := r.Header.Get(HeaderAcceptLanguage)

			// Find the "best" language from the given parameters. They are in
			// priority order.
			m := controller.TemplateMapFromContext(ctx)
			m["locale"] = locales.Lookup(param, header)

			// Save the template map on the context.
			ctx = controller.WithTemplateMap(ctx, m)
			r = r.Clone(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
