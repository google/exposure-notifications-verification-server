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

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestRequireAPIKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	db := harness.Database
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	cacher, err := cache.NewNoop()
	if err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		Name:       "Appy",
		APIKeyType: database.APIKeyTypeAdmin,
	}
	apiKey, err := realm.CreateAuthorizedApp(db, authApp, database.SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	wrongAuthApp := &database.AuthorizedApp{
		Name:       "Statsy",
		APIKeyType: database.APIKeyTypeStats,
	}
	wrongAPIKey, err := realm.CreateAuthorizedApp(db, wrongAuthApp, database.SystemTest)
	if err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	badDB, _ := testDatabaseInstance.NewDatabase(t, nil)
	badDB.SetRawDB(envstest.NewFailingDatabase())

	cases := []struct {
		name   string
		apiKey string
		code   int

		db   *database.Database
		next func(t *testing.T) http.Handler
	}{
		{
			name:   "no_key",
			apiKey: "",
			code:   401,
			db:     db,
		},
		{
			name:   "non_existent_key",
			apiKey: "abcd1234",
			code:   401,
			db:     db,
		},
		{
			name:   "bad_database_conn",
			apiKey: apiKey,
			code:   500,
			db:     badDB,
		},
		{
			name:   "wrong_type",
			apiKey: wrongAPIKey,
			code:   401,
			db:     db,
		},
		{
			name:   "valid",
			apiKey: apiKey,
			code:   200,
			db:     db,
			next: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()

					authApp := controller.AuthorizedAppFromContext(ctx)
					if authApp == nil {
						t.Errorf("expected auth app in context")
					}

					realm := controller.RealmFromContext(ctx)
					if realm == nil {
						t.Errorf("expected realm in context")
					}
				})
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest("GET", "/", nil)
			r = r.Clone(ctx)
			r.Header.Set(middleware.APIKeyHeader, tc.apiKey)
			r.Header.Set("Accept", "application/json")

			next := emptyHandler()
			if tc.next != nil {
				next = tc.next(t)
			}

			w := httptest.NewRecorder()
			handler := middleware.RequireAPIKey(cacher, tc.db, h, []database.APIKeyType{
				database.APIKeyTypeAdmin,
			})(next)

			handler.ServeHTTP(w, r)
			w.Flush()

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}
