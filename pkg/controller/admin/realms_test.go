// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleRealmsIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleRealmsIndex())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseUserMissing(t, handler)
		envstest.ExerciseBadPagination(t, &database.Membership{
			User: &database.User{},
		}, handler)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleRealmsIndex())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
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
}

func TestHandleRealmsCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleRealmsCreate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleRealmsCreate())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
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
			"name": []string{""},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
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

	t.Run("creates", func(t *testing.T) {
		t.Parallel()

		user, err := harness.Database.FindUser(1)
		if err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":                        []string{"realmy"},
			"regionCode":                  []string{"TT-tt"},
			"useRealmCertificateKey":      []string{"1"},
			"certificateIssuer":           []string{"iss"},
			"certificateAudience":         []string{"aud"},
			"can_use_system_sms_config":   []string{"1"},
			"can_use_system_email_config": []string{"1"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/realms/2/edit"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}

func TestHandleRealmsUpdate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleRealmsUpdate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleRealmsUpdate())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("renders", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"can_use_system_sms_config":   []string{"1"},
			"can_use_system_email_config": []string{"1"},
			"short_code_max_minutes":      []string{"60"},
		})
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/admin/realms/1/edit"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}

func TestHandleRealmsAdd(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleRealmsAdd())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("realm_id_not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "12345",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("user_id_not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "12345",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleRealmsAdd())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("adds", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/back"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		memberships, err := realm.MembershipPermissionMap(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		perms, ok := memberships[1]
		if !ok {
			t.Errorf("expected user to be added to realm")
		}
		if got, want := perms, rbac.LegacyRealmAdmin; !rbac.Can(got, want) {
			t.Errorf("expected %q to be able to %q", got, want)
		}
	})
}

func TestHandleRealmsRemove(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleRealmsRemove())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("realm_id_not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "12345",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("user_id_not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "12345",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleRealmsRemove())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("removes", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"realm_id": "1",
			"user_id":  "1",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/back"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		memberships, err := realm.MembershipPermissionMap(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := memberships[1]; ok {
			t.Errorf("expected user to be remove from realm")
		}
	})
}
