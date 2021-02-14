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

package realmadmin_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestHandleEnableExpress(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	_, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleEnableExpress()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleEnableExpress()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				EnableENExpress: false,
			},
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("already_enabled", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleEnableExpress()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				EnableENExpress: true,
			},
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "is already enrolled in EN Express"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleEnableExpress()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				EnableENExpress: false,
			},
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("enables_express", func(t *testing.T) {
		t.Parallel()

		realm := database.NewRealmWithDefaults("realmy2")
		realm.RegionCode = "TT"
		realm.EnableENExpress = false
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Errorf("%#v", realm.ErrorMessages())
			t.Fatal(err)
		}

		c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, h)
		handler := c.HandleEnableExpress()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("Expected %d to be %d", got, want)
			t.Errorf("%#v", realm.ErrorMessages())
		}

		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.EnableENExpress, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
	})
}
