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

package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestRoutes_systemAdminRoutes(t *testing.T) {
	t.Parallel()

	r := mux.NewRouter()
	systemAdminRoutes(r, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realms", nil),
		},
		{
			req: httptest.NewRequest("POST", "/realms", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realms/new", nil),
		},
		{
			req:  httptest.NewRequest("GET", "/realms/12345/edit", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest("PATCH", "/realms/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest("PATCH", "/realms/12345/add/67890", nil),
			vars: map[string]string{"realm_id": "12345", "user_id": "67890"},
		},
		{
			req:  httptest.NewRequest("PATCH", "/realms/12345/remove/67890", nil),
			vars: map[string]string{"realm_id": "12345", "user_id": "67890"},
		},
		{
			req:  httptest.NewRequest("GET", "/realms/12345/realmadmin", nil), // TODO: better route
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest("GET", "/users", nil),
		},
		{
			req:  httptest.NewRequest("GET", "/users/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest("DELETE", "/users/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest("POST", "/users", nil),
		},
		{
			req: httptest.NewRequest("GET", "/users/new", nil),
		},
		{
			req:  httptest.NewRequest("DELETE", "/users/12345/revoke", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest("GET", "/mobileapps", nil),
		},
		{
			req: httptest.NewRequest("GET", "/sms", nil),
		},
		{
			req: httptest.NewRequest("GET", "/email", nil),
		},
		{
			req: httptest.NewRequest("GET", "/events", nil),
		},
		{
			req: httptest.NewRequest("GET", "/caches", nil),
		},
		{
			req:  httptest.NewRequest("POST", "/caches/clear/banana", nil),
			vars: map[string]string{"id": "banana"},
		},
		{
			req: httptest.NewRequest("GET", "/info", nil),
		},
	}

	for _, tc := range cases {
		tc := tc

		pth := strings.Replace(strings.Trim(tc.req.URL.Path, "/"), "/", "_", -1)
		name := fmt.Sprintf("%s_%s", tc.req.Method, pth)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var match mux.RouteMatch
			ok := r.Match(tc.req, &match)
			if !ok {
				t.Fatalf("expected route to match: %v", match.MatchErr)
			}

			for k, want := range tc.vars {
				got, ok := match.Vars[k]
				if !ok {
					t.Errorf("expected vars to contain %q", k)
				}

				if got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
			}
		})
	}
}
