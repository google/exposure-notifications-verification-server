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

package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealHostFromRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		req   *http.Request
		given string
		exp   string
	}{
		{
			name: "default",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			exp:  "https://example.com",
		},
		{
			name: "localhost",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Host = "localhost"
				return r
			}(),
			exp: "http://localhost",
		},
		{
			name: "custom_port",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				r.Host = "localhost:8080"
				return r
			}(),
			exp: "http://localhost:8080",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := RealHostFromRequest(tc.req), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
