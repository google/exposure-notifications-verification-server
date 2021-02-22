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
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
)

func TestHandleShow(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, testUser, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithSession(ctx, session)

	c := user.New(harness.AuthProvider, harness.Cacher, harness.Database, harness.Renderer)
	handler := c.HandleShow()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			Realm:       realm,
			User:        testUser,
			Permissions: rbac.UserRead,
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := user.New(harness.AuthProvider, harness.Cacher, harness.BadDatabase, harness.Renderer)
		handler := c.HandleShow()

		ctx := ctx
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        testUser,
			Permissions: rbac.UserRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"id": fmt.Sprintf("%d", testUser.ID),
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        testUser,
			Permissions: rbac.UserRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{
			"id": fmt.Sprintf("%d", testUser.ID),
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}
