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
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestAdminCaches(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleCachesClear()))

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		session := &sessions.Session{}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/back"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		errs := flash.Errors()
		if got, want := len(errs), 1; got != want {
			t.Errorf("Expected %d errors, got %d", want, got)
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

		c := admin.New(harness.Config, cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleCachesClear()))

		session := &sessions.Session{}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "realms:"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/back"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		errs := flash.Errors()
		if got, want := len(errs), 1; got != want {
			t.Errorf("Expected %d errors, got %d", want, got)
		}
		if got, want := errs[0], "Failed to clear cache"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("clears", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "realms:"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/back"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}
