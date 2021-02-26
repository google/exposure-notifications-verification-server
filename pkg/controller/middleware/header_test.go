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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestRequireHeader(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	requireHeader := middleware.RequireHeader("x-custom-header", h)

	cases := []struct {
		name    string
		headers map[string]string
		code    int
	}{
		{
			name: "missing",
			code: 401,
		},
		{
			name:    "present",
			headers: map[string]string{"x-custom-header": "1"},
			code:    200,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Accept", "application/json")
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			requireHeader(emptyHandler()).ServeHTTP(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestRequireHeaderValues(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	allowed := []string{"1", "2"}
	requireHeaderValues := middleware.RequireHeaderValues("x-custom-header", allowed, h)

	cases := []struct {
		name    string
		headers map[string]string
		code    int
	}{
		{
			name: "missing",
			code: 401,
		},
		{
			name:    "present_invalid",
			headers: map[string]string{"x-custom-header": "42"},
			code:    401,
		},
		{
			name:    "present_valid",
			headers: map[string]string{"x-custom-header": "1"},
			code:    200,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Accept", "application/json")
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}

			w := httptest.NewRecorder()

			requireHeaderValues(emptyHandler()).ServeHTTP(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}
