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

package admin_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleSystemAdminCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleSystemAdminCreate())
		mux.Handle("/", c.HandleSystemAdminCreate())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)
		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleSystemAdminCreate()).Methods(http.MethodPost)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader((&url.Values{
			"name":  []string{"Tester"},
			"email": []string{fmt.Sprintf("tester-%s@example.com", suffix)},
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleSystemAdminCreate()).Methods(http.MethodPost)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader((&url.Values{
			"name":  []string{"Tester"},
			"email": []string{""}, // blank
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %s to contain %s", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleSystemAdminCreate()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("creates", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleSystemAdminCreate()).Methods(http.MethodPost)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader((&url.Values{
			"name":  []string{"Tester"},
			"email": []string{fmt.Sprintf("tester-%s@example.com", suffix)},
		}).Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})
}

func TestHandleSystemAdminRevoke(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleSystemAdminRevoke())
		mux.Handle("/", c.HandleSystemAdminRevoke())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			User: &database.User{},
		}, mux)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)
		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleSystemAdminRevoke()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("revoke_self", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}
		user := &database.User{
			Name:        "Tester",
			Email:       fmt.Sprintf("tester-%s@example.com", suffix),
			SystemAdmin: true,
		}
		if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleSystemAdminRevoke()).Methods(http.MethodGet)

		session := &sessions.Session{
			Values: make(map[interface{}]interface{}),
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)

		u := fmt.Sprintf("/%d", user.ID)
		r := httptest.NewRequest(http.MethodGet, u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		if got, want := strings.Join(flash.Errors(), ", "), "Cannot remove yourself"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("revokes", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}
		user := &database.User{
			Name:        "Tester",
			Email:       fmt.Sprintf("tester-%s@example.com", suffix),
			SystemAdmin: true,
		}
		if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleSystemAdminRevoke()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		u := fmt.Sprintf("/%d", user.ID)
		r := httptest.NewRequest(http.MethodGet, u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		updatedUser, err := harness.Database.FindUser(user.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := updatedUser.SystemAdmin, false; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
	})
}

func TestHandleUserDelete(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	middlewares := []mux.MiddlewareFunc{
		middleware.InjectCurrentPath(),
		middleware.ProcessLocale(locales),
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleUserDelete())
		mux.Handle("/", c.HandleUserDelete())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseUserMissing(t, mux)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			User: &database.User{},
		}, mux)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)
		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleUserDelete()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("delete_self", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}
		user := &database.User{
			Name:        "Tester",
			Email:       fmt.Sprintf("tester-%s@example.com", suffix),
			SystemAdmin: true,
		}
		if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleUserDelete()).Methods(http.MethodGet)

		session := &sessions.Session{
			Values: make(map[interface{}]interface{}),
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)

		u := fmt.Sprintf("/%d", user.ID)
		r := httptest.NewRequest(http.MethodGet, u, nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		if got, want := strings.Join(flash.Errors(), ", "), "Cannot delete yourself"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("deletes", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleUserDelete()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})
}
