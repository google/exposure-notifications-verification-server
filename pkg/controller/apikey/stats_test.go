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

package apikey_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleStats(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		RealmID: realm.ID,
		Name:    "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	h, err := render.New(context.Background(), envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}
	c := apikey.New(harness.Cacher, harness.Database, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, c.HandleStats())
		envstest.ExerciseMembershipMissing(t, c.HandleStats())
		envstest.ExercisePermissionMissing(t, c.HandleStats())
	})

	t.Run("internal_error", func(t *testing.T) {
		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		h, err := render.New(context.Background(), envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := apikey.New(harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/{id}", c.HandleStats()).Methods("GET")

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.APIKeyRead,
		})

		r := httptest.NewRequest("GET", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("expected %s to contain %q", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Handle("/{id}.json", c.HandleStats())
		mux.Handle("/{id}.csv", c.HandleStats())

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.APIKeyRead,
		})

		r := httptest.NewRequest("GET", "/100.json", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "unauthorized"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("unknown_extension", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Handle("/{id}.xml", c.HandleStats())

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.APIKeyRead,
		})

		u := fmt.Sprintf("/%d.xml", authApp.ID)
		r := httptest.NewRequest("GET", u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 404; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Not found"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Handle("/{id}.json", c.HandleStats())
		mux.Handle("/{id}.csv", c.HandleStats())

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.APIKeyRead,
		})

		u := fmt.Sprintf("/%d.json", authApp.ID)
		r := httptest.NewRequest("GET", u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "codes_issued"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("csv", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Handle("/{id}.json", c.HandleStats())
		mux.Handle("/{id}.csv", c.HandleStats())

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.APIKeyRead,
		})

		u := fmt.Sprintf("/%d.csv", authApp.ID)
		r := httptest.NewRequest("GET", u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "codes_issued"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})
}
