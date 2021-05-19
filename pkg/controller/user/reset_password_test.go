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

package user_test

import (
	"fmt"
	"net/http"
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

func TestHandleResetPassword(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := user.New(harness.AuthProvider, harness.Cacher, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleResetPassword())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		admin, testUser, realm := provisionUsers(t, harness.Database)

		c := user.New(harness.AuthProvider, harness.Cacher, harness.BadDatabase, harness.Renderer)
		handler := c.HandleResetPassword()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.UserWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", testUser.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d", got, want)
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
			Permissions: rbac.UserWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", testUser.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})
}
