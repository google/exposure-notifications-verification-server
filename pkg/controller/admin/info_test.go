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
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/gorilla/sessions"
)

func TestAdminInfo(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	locales, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleEventsShow()))

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := middleware.InjectCurrentPath()(middleware.ProcessLocale(locales)(c.HandleEventsShow()))

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, w.Header())
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, w.Header())
		}
	})
}
