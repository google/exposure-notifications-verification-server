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

// This goes to the value of a <input type="datetime-local">
const rfc3339PartialLocal = "2006-01-02T15:04:05"

func TestAdminEvents(t *testing.T) {
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
		mux.Handle("/{id}", c.HandleEventsShow())
		mux.Handle("/", c.HandleEventsShow())

		envstest.ExerciseSessionMissing(t, mux)
		envstest.ExerciseBadPagination(t, &database.Membership{}, mux)
	})

	t.Run("list_internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleEventsShow()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("list_realm_internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleEventsShow()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/?realm_id=1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_all", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleEventsShow()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_system", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Use(middlewares...)
		mux.Handle("/", c.HandleEventsShow()).Methods(http.MethodGet)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		r := httptest.NewRequest(http.MethodGet, "/?realm_id=0", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusOK; got != want {
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

		eventTime, err := time.Parse(time.RFC3339, "2020-03-11T12:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		audit := &database.AuditEntry{
			RealmID:       0, // system entry
			Action:        "test action",
			TargetID:      "testTargetID",
			TargetDisplay: "test target",
			ActorID:       "testActorID",
			ActorDisplay:  "test actor",
			CreatedAt:     eventTime,
		}
		harness.Database.RawDB().Save(audit)

		// Create a browser runner.
		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
		defer done()

		if err := chromedp.Run(taskCtx,
			// Pre-authenticate the user.
			browser.SetCookie(cookie),

			// Visit /admin
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/events`),

			// Wait for render.
			chromedp.WaitVisible(`body#admin-events-index`, chromedp.ByQuery),

			// Search from and hour before to and hour after our event
			chromedp.SetValue(`#from`, eventTime.Add(-time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
			chromedp.SetValue(`#to`, eventTime.Add(time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
			chromedp.Submit(`form#search-form`, chromedp.ByQuery),

			// Wait for the search result.
			chromedp.WaitVisible(`#results #event`, chromedp.ByQuery),

			// Search an hour before the event.
			chromedp.SetValue(`#from`, eventTime.Add(-2*time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
			chromedp.SetValue(`#to`, eventTime.Add(-time.Hour).Format(rfc3339PartialLocal), chromedp.ByQuery),
			chromedp.Submit(`form#search-form`, chromedp.ByQuery),

			// Assert no event found
			chromedp.WaitNotPresent(`#results #event`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}
	})
}
