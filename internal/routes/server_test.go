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
			req: httptest.NewRequest(http.MethodGet, "/issue", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/bulk-issue", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/status", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/aaa-aaa-aaa-aaa", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/aaa-aaa-aaa-aaa/expire", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/new", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345/disable", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345/enable", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/new", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345/disable", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345/enable", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/new", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/import", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/import", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345/edit", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPatch, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodDelete, "/12345", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/12345/reset-password", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/keys", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodDelete, "/keys/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/keys/create", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/keys/upgrade", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/keys/save", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/keys/activate", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/sms-keys", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/sms-keys", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPut, "/sms-keys/enable", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPut, "/sms-keys/disable", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/sms-keys/activate", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodDelete, "/sms-keys/12345", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/realm.csv", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm.json", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/users.csv", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/users.json", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/users/12345.csv", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/users/12345.json", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/api-keys/12345.csv", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/api-keys/12345.json", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/external-issuers.csv", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realm/external-issuers.json", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/settings", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/settings", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/settings/enable-express", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/settings/disable-express", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/stats", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/events", nil),
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
			req:  httptest.NewRequest(http.MethodGet, "/12345", nil),
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
			req: httptest.NewRequest(http.MethodGet, "/", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realms", nil),
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/realms", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/realms/new", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodGet, "/realms/12345/edit", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest(http.MethodPatch, "/realms/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest(http.MethodPatch, "/realms/12345/add/67890", nil),
			vars: map[string]string{"realm_id": "12345", "user_id": "67890"},
		},
		{
			req:  httptest.NewRequest(http.MethodPatch, "/realms/12345/remove/67890", nil),
			vars: map[string]string{"realm_id": "12345", "user_id": "67890"},
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/users", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodGet, "/users/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req:  httptest.NewRequest(http.MethodDelete, "/users/12345", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest(http.MethodPost, "/users", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/users/new", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodDelete, "/users/12345/revoke", nil),
			vars: map[string]string{"id": "12345"},
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/mobile-apps", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/sms", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/email", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/events", nil),
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/caches", nil),
		},
		{
			req:  httptest.NewRequest(http.MethodPost, "/caches/clear/banana", nil),
			vars: map[string]string{"id": "banana"},
		},
		{
			req: httptest.NewRequest(http.MethodGet, "/info", nil),
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
