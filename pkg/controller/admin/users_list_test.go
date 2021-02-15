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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
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

	"github.com/chromedp/chromedp"
)

func TestAdminUsersIndex(t *testing.T) {
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
		mux.Handle("/{id}", c.HandleUsersIndex())
		mux.Handle("/", c.HandleUsersIndex())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseBadPagination(t, &database.Membership{}, mux)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)
		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleUsersIndex()).Methods(http.MethodGet)

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
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleUsersIndex()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_sysadmins", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleUsersIndex()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/?filter=systemAdmins", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		_, _, session, err := harness.ProvisionAndLogin()
		if err != nil {
			t.Fatal(err)
		}

		cookie, err := harness.SessionCookie(session)
		if err != nil {
			t.Fatal(err)
		}

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/users`),
			chromedp.WaitVisible(`body#admin-users-index`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAdminUserShow(t *testing.T) {
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
		mux.Handle("/{id}", c.HandleUserShow())
		mux.Handle("/", c.HandleUserShow())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseIDNotFound(t, &database.Membership{}, mux)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)
		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleUserShow()).Methods(http.MethodGet)

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
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("shows", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleUserShow()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		_, admin, session, err := harness.ProvisionAndLogin()
		if err != nil {
			t.Fatal(err)
		}

		cookie, err := harness.SessionCookie(session)
		if err != nil {
			t.Fatal(err)
		}

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		var email string
		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),
			chromedp.Navigate(fmt.Sprintf(`http://`+harness.Server.Addr()+`/admin/users/%d`, admin.ID)),
			chromedp.WaitVisible(`body#admin-users-show`, chromedp.ByQuery),
			chromedp.Text(`#user-email`, &email, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}

		if got, want := email, admin.Email; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}
