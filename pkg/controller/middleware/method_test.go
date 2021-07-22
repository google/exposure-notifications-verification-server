// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestMutateMethod(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	mutateMethod := middleware.MutateMethod()

	cases := []struct {
		name string
		r    io.Reader
		exp  string
	}{
		{
			name: "no_body",
			r:    nil,
			exp:  http.MethodPost,
		},
		{
			name: "no_key",
			r: strings.NewReader(url.Values{
				"foo": []string{"bar"},
			}.Encode()),
			exp: http.MethodPost,
		},
		{
			name: "overrides",
			r: strings.NewReader(url.Values{
				"_method": []string{"CUSTOM"},
			}.Encode()),
			exp: "CUSTOM",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodPost, "/", tc.r)
			r = r.Clone(ctx)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			mutateMethod(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.Method, tc.exp; got != want {
					t.Errorf("Expected %q to be %q", got, want)
				}
			})).ServeHTTP(w, r)
		})
	}
}
