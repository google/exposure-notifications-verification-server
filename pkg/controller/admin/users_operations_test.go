// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleSystemAdminCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleSystemAdminCreate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSystemAdminCreate())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":  []string{"Tester"},
			"email": []string{"tester@example.com"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":  []string{"Tester"},
			"email": []string{""}, // blank
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %s to contain %s", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		suffix, err := project.RandomHexString(6)
		if err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":  []string{"Tester"},
			"email": []string{fmt.Sprintf("tester-%s@example.com", suffix)},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}

func TestHandleSystemAdminRevoke(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleSystemAdminRevoke())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			User: &database.User{},
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSystemAdminRevoke())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
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

		session := &sessions.Session{}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", user.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		if got, want := strings.Join(flash.Errors(), ", "), "Cannot remove yourself"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
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

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", user.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
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
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleUserDelete())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			User: &database.User{},
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleUserDelete())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodDelete, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
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

		session := &sessions.Session{}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", user.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		flash := controller.Flash(session)
		if got, want := strings.Join(flash.Errors(), ", "), "Cannot delete yourself"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("deletes", func(t *testing.T) {
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

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", user.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/users"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		if _, err := harness.Database.FindUser(user.ID); !database.IsNotFound(err) {
			t.Fatal(err)
		}
	})
}
