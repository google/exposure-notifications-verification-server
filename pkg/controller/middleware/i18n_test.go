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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/leonelquinteros/gotext"
)

func TestProcessLocale(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	locales, err := i18n.Load(filepath.Join(project.Root(), "internal", "i18n", "locales"))
	if err != nil {
		t.Fatal(err)
	}

	processLocale := middleware.ProcessLocale(locales)

	cases := []struct {
		name    string
		query   string
		headers map[string]string
		exp     string
	}{
		{
			name:  "param",
			query: "?lang=es",
			exp:   "Generar código",
		},
		{
			name: "header",
			headers: map[string]string{
				"Accept-Language": "es",
			},
			exp: "Generar código",
		},
		{
			name: "none",
			exp:  "Issue code",
		},
		{
			name:  "missing",
			query: "?lang=not-a-real-language-nope",
			exp:   "Issue code",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/"+tc.query, nil)
			r = r.Clone(ctx)
			r.Header.Set("Accept", "application/json")
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			processLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				m := controller.TemplateMapFromContext(ctx)

				v, ok := m["locale"]
				if !ok {
					t.Fatalf("expected locale to be populated in template map")
				}

				locale, ok := v.(*gotext.Locale)
				if !ok {
					t.Fatalf("expected %T to be *gotext.Locale", v)
				}

				if got, want := locale.Get("nav.issue-code"), tc.exp; got != want {
					t.Errorf("Expected %q to be %q", got, want)
				}
			})).ServeHTTP(w, r)
		})
	}
}
