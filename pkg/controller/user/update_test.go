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

package user_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := user.New(harness.AuthProvider, harness.Cacher, harness.Database, harness.Renderer)
	handler := c.HandleUpdate()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.UserWrite,
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := user.New(harness.AuthProvider, harness.Cacher, harness.BadDatabase, harness.Renderer)
		handler := c.HandleUpdate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name": []string{"apple"},
		})
		r = mux.SetURLVars(r, map[string]string{"id": "123456789"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		admin, testUser, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name": []string{""},
		})
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", testUser.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		admin, testUser, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name": []string{testUser.Name},
			"permissions": []string{
				fmt.Sprintf("%d", rbac.APIKeyRead),
				fmt.Sprintf("%d", rbac.UserWrite),
			},
		})
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", testUser.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}

		permission, err := testUser.FindMembership(harness.Database, realm.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range []rbac.Permission{
			rbac.APIKeyRead,
			rbac.UserRead,
			rbac.UserWrite,
		} {
			if !permission.Can(p) {
				t.Errorf("expected %q to be able to %q", permission.Permissions, p)
			}
		}
		if p := rbac.StatsRead; permission.Can(p) {
			t.Errorf("expected %q to not be able to %q", permission.Permissions, p)
		}
	})
}
