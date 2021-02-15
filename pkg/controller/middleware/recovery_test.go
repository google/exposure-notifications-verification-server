// Copyright 2021 Google LLC
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

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestRecovery(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	m := middleware.Recovery(h)

	cases := []struct {
		name    string
		handler http.Handler
		code    int
	}{
		{
			name:    "default",
			handler: emptyHandler(),
			code:    200,
		},
		{
			name: "panic",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("oops")
			}),
			code: 500,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			w := httptest.NewRecorder()

			m(tc.handler).ServeHTTP(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("expected %d to be %d", got, want)
			}
		})
	}
}
