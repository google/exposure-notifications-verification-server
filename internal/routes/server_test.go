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
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/keys"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/gorilla/mux"
	"github.com/sethvargo/go-limiter/memorystore"
)

func TestServer(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	cfg := &config.ServerConfig{
		LocalesPath: filepath.Join(project.Root(), "internal", "i18n", "locales"),
	}
	db := &database.Database{}
	cacher, err := cache.NewNoop()
	if err != nil {
		t.Fatal(err)
	}

	authProvider, err := auth.NewLocal(ctx)
	if err != nil {
		t.Fatal(err)
	}

	limiterStore, err := memorystore.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	signer := keys.TestKeyManager(t)

	mux, err := Server(ctx, cfg, db, authProvider, cacher, signer, signer, limiterStore)
	if err != nil {
		t.Fatal(err)
	}
	_ = mux
}

func TestRoutes_codesRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	codesRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/issue", nil),
		},
		{
			req: httptest.NewRequest("GET", "/bulk-issue", nil),
		},
		{
			req: httptest.NewRequest("GET", "/status", nil),
		},
		{
			req: httptest.NewRequest("GET", "/aaa-aaa-aaa-aaa", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/aaa-aaa-aaa-aaa/expire", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_mobileappsRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	mobileappsRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/new", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345/disable", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345/enable", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_apikeyRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	apikeyRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/new", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345/disable", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345/enable", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_userRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	userRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/new", nil),
		},
		{
			req: httptest.NewRequest("GET", "/import", nil),
		},
		{
			req: httptest.NewRequest("POST", "/import", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest("GET", "/12345", nil),
		},
		{
			req: httptest.NewRequest("PATCH", "/12345", nil),
		},
		{
			req: httptest.NewRequest("DELETE", "/12345", nil),
		},
		{
			req: httptest.NewRequest("POST", "/12345/reset-password", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_realmkeysRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	realmkeysRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/keys", nil),
		},
		{
			req:  httptest.NewRequest("DELETE", "/keys/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest("POST", "/keys/create", nil),
		},
		{
			req: httptest.NewRequest("POST", "/keys/upgrade", nil),
		},
		{
			req: httptest.NewRequest("POST", "/keys/save", nil),
		},
		{
			req: httptest.NewRequest("POST", "/keys/activate", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_realmSMSkeysRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	realmSMSkeysRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/sms-keys", nil),
		},
		{
			req: httptest.NewRequest("POST", "/sms-keys", nil),
		},
		{
			req: httptest.NewRequest("PUT", "/sms-keys/enable", nil),
		},
		{
			req: httptest.NewRequest("PUT", "/sms-keys/disable", nil),
		},
		{
			req: httptest.NewRequest("POST", "/sms-keys/activate", nil),
		},
		{
			req:  httptest.NewRequest("DELETE", "/sms-keys/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_statsRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	statsRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/realm.csv", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm.json", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/users.csv", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/users.json", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/users/12345.csv", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/users/12345.json", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/api-keys/12345.csv", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/api-keys/12345.json", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/external-issuers.csv", nil),
		},
		{
			req: httptest.NewRequest("GET", "/realm/external-issuers.json", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_realmadminRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	realmadminRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req: httptest.NewRequest("GET", "/settings", nil),
		},
		{
			req: httptest.NewRequest("POST", "/settings", nil),
		},
		{
			req: httptest.NewRequest("POST", "/settings/enable-express", nil),
		},
		{
			req: httptest.NewRequest("POST", "/settings/disable-express", nil),
		},
		{
			req: httptest.NewRequest("GET", "/stats", nil),
		},
		{
			req: httptest.NewRequest("GET", "/events", nil),
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_jwksRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	jwksRoutes(m, nil)

	cases := []struct {
		req  *http.Request
		vars map[string]string
	}{
		{
			req:  httptest.NewRequest("GET", "/12345", nil),
			vars: map[string]string{"realm_id": "12345"},
		},
	}

	for _, tc := range cases {
		testRoute(t, m, tc.req, tc.vars)
	}
}

func TestRoutes_systemAdminRoutes(t *testing.T) {
	t.Parallel()

	m := mux.NewRouter()
	systemAdminRoutes(m, nil)

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
			req: httptest.NewRequest("GET", "/mobile-apps", nil),
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
		testRoute(t, m, tc.req, tc.vars)
	}
}

func testRoute(t *testing.T, m *mux.Router, r *http.Request, vars map[string]string) {
	pth := strings.Replace(strings.Trim(r.URL.Path, "/"), "/", "_", -1)
	name := fmt.Sprintf("%s_%s", r.Method, pth)

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		var match mux.RouteMatch
		ok := m.Match(r, &match)
		if !ok {
			t.Fatalf("expected route to match: %v", match.MatchErr)
		}

		for k, want := range vars {
			got, ok := match.Vars[k]
			if !ok {
				t.Errorf("expected vars to contain %q", k)
			}

			if got != want {
				t.Errorf("Expected %q to be %q", got, want)
			}
		}
	})
}
