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

	"github.com/chromedp/chromedp"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestAdminMobileApps(t *testing.T) {
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
		mux.Handle("/{id}", c.HandleMobileAppsShow())
		mux.Handle("/", c.HandleMobileAppsShow())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseBadPagination(t, &database.Membership{}, mux)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleMobileAppsShow()).Methods(http.MethodGet)

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

	t.Run("lists_all", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleMobileAppsShow()).Methods(http.MethodGet)

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
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		realm, _, session, err := harness.ProvisionAndLogin()
		if err != nil {
			t.Fatal(err)
		}

		cookie, err := harness.SessionCookie(session)
		if err != nil {
			t.Fatal(err)
		}

		app := &database.MobileApp{
			Name:    "test mobile app",
			RealmID: realm.ID,
			URL:     "https://example2.com",
			OS:      database.OSTypeAndroid,
			SHA:     "AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA",
			AppID:   "app2",
		}
		if err := harness.Database.SaveMobileApp(app, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),

			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/mobile-apps`),
			chromedp.WaitVisible(`body#admin-mobileapps-index`, chromedp.ByQuery),

			chromedp.SetValue(`input#search`, "test mobile app", chromedp.ByQuery),
			chromedp.Submit(`form#search-form`, chromedp.ByQuery),
			chromedp.WaitVisible(`table#results-table tr`, chromedp.ByQuery),

			chromedp.SetValue(`input#search`, "notexists", chromedp.ByQuery),
			chromedp.Submit(`form#search-form`, chromedp.ByQuery),

			chromedp.WaitNotPresent(`table#results-table tr`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	})
}
