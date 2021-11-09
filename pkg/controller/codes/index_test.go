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

package codes_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandleIndex_ShowRecentCodes(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := codes.NewServer(harness.Config, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleIndex())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := codes.NewServer(harness.Config, harness.BadDatabase, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleIndex())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.CodeRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
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

		code := &database.VerificationCode{
			RealmID:       realm.ID,
			Code:          "00000001",
			LongCode:      "00000001ABC",
			Claimed:       true,
			TestType:      "confirmed",
			ExpiresAt:     time.Now().Add(time.Hour),
			LongExpiresAt: time.Now().Add(time.Hour),
		}
		if err := realm.SaveVerificationCode(harness.Database, code); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.CodeRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})
}
