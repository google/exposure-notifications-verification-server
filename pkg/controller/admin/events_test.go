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
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// This goes to the value of a <input type="datetime-local">
const rfc3339PartialLocal = "2006-01-02T15:04:05"

func TestAdminEvents(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleEventsShow()))

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseBadPagination(t, &database.Membership{}, handler)
	})

	t.Run("list_internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleEventsShow()))

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("list_realm_internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleEventsShow()))

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"realm_id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_all", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("lists_system", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"realm_id": "0"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}
