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

package admin_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestAdminCaches(t *testing.T) {
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
		mux.Handle("/{id}", c.HandleCachesClear())
		mux.Handle("/", c.HandleCachesClear())

		envstest.ExerciseSessionMissing(t, mux)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleCachesClear()).Methods("PUT")

		session := &sessions.Session{
			Values: map[interface{}]interface{}{},
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)

		r := httptest.NewRequest("PUT", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")
		r.Header.Set("Referer", "https://example.com/foo/bar")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "https://example.com/foo/bar"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		errs := flash.Errors()
		if got, want := len(errs), 1; got != want {
			t.Errorf("expected %d errors, got %d", want, got)
		}
		if got, want := errs[0], "Unknown cache type"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("cacher_failure", func(t *testing.T) {
		t.Parallel()

		cacher, err := cache.NewInMemory(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := cacher.Close(); err != nil {
			t.Fatal(err)
		}

		c := admin.New(harness.Config, cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleCachesClear()).Methods("PUT")

		session := &sessions.Session{
			Values: map[interface{}]interface{}{},
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)

		r := httptest.NewRequest("PUT", "/realms:", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")
		r.Header.Set("Referer", "https://example.com/foo/bar")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "https://example.com/foo/bar"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		errs := flash.Errors()
		if got, want := len(errs), 1; got != want {
			t.Errorf("expected %d errors, got %d", want, got)
		}
		if got, want := errs[0], "Failed to clear cache"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("clears", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/{id}", c.HandleCachesClear()).Methods("PUT")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		r := httptest.NewRequest("PUT", "/realms:", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")
		r.Header.Set("Referer", "https://example.com/foo/bar")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "https://example.com/foo/bar"; got != want {
			t.Errorf("expected %q to be %q", got, want)
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

		// Create a browser runner.
		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		if err := chromedp.Run(taskCtx,
			// Pre-authenticate the user.
			browser.SetCookie(cookie),

			// Visit /admin
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/caches`),

			// Wait for render.
			chromedp.WaitVisible(`body#admin-caches-index`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	})
}
