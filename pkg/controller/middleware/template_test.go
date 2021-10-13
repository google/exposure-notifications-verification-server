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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
)

func TestPopulateTemplateVariables(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.ServerConfig{
		ServerName:      "namey",
		ServerEndpoint:  "https://foo.bar",
		MaintenanceMode: true,
	}
	if err := cfg.Process(ctx); err != nil {
		t.Fatal(err)
	}
	populateTemplateVariables := middleware.PopulateTemplateVariables(cfg)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.Clone(ctx)

	w := httptest.NewRecorder()

	// Verify the proper fields are added to the template map.
	populateTemplateVariables(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := controller.TemplateMapFromContext(ctx)

		if got, want := m["server"], cfg.ServerName; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
		if got, want := m["serverEndpoint"], cfg.ServerEndpoint; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
		if got, want := m["title"], cfg.ServerName; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
		if _, ok := m["buildID"]; !ok {
			t.Errorf("expected buildID to be populated in template map")
		}
		if _, ok := m["buildTag"]; !ok {
			t.Errorf("expected buildTag to be populated in template map")
		}
		if got, want := m["systemNotice"].(string), "undergoing maintenance"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})).ServeHTTP(w, r)
}

func TestServerEndpoint(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

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

			var cfg config.ServerConfig
			populateTemplateVariables := middleware.PopulateTemplateVariables(&cfg)

			r := tc.req.Clone(ctx)
			w := httptest.NewRecorder()

			// Verify the proper fields are added to the template map.
			populateTemplateVariables(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				m := controller.TemplateMapFromContext(ctx)

				if got, want := m["serverEndpoint"], tc.exp; got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
			})).ServeHTTP(w, r)
		})
	}
}
