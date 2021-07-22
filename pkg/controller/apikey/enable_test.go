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

package apikey_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
)

func TestHandleEnable(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := apikey.New(harness.Cacher, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleEnable())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.APIKeyWrite,
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := apikey.New(harness.Cacher, harness.BadDatabase, harness.Renderer)
		handler := c.HandleEnable()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC().Add(-5 * time.Second)
		authApp := &database.AuthorizedApp{
			RealmID: realm.ID,
			Name:    "Appy1",
			Model: gorm.Model{
				DeletedAt: &now,
			},
		}
		if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.APIKeyWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", authApp.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC().Add(-5 * time.Second)
		authApp := &database.AuthorizedApp{
			RealmID: realm.ID,
			Name:    "Appy2",
			Model: gorm.Model{
				DeletedAt: &now,
			},
		}
		if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.APIKeyWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", authApp.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		// Ensure enabled
		record, err := harness.Database.FindAuthorizedApp(authApp.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := record.DeletedAt; got != nil {
			t.Errorf("expected %v to be nil", got)
		}
	})
}
