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

package user_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func TestHandleCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := user.New(harness.AuthProvider, harness.Cacher, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleCreate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := user.New(harness.AuthProvider, harness.Cacher, harness.BadDatabase, harness.Renderer)
		handler := c.HandleCreate()

		admin, _, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":        []string{"person"},
			"email":       []string{"you@example.com"},
			"permissions": []string{"2", "4", "8"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("expected %s to contain %q", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		admin, _, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		admin, _, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":  []string{"Test User"},
			"email": []string{"you@example.com"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/realm/users/"; !strings.HasPrefix(got, want) {
			t.Errorf("expected %q to start with %q", got, want)
		}

		records, _, err := realm.ListMemberships(harness.Database, pagination.UnlimitedResults)
		if err != nil {
			t.Fatal(err)
		}

		var record *database.User
		for _, r := range records {
			if r.User.Name == "Test User" {
				record = r.User
				break
			}
		}
		if record == nil {
			t.Fatalf("failed to find record: %#v", records)
		} else {
			// The static check, SA5011, is not detecting the nil check correctly here.
			if got, want := record.Name, "Test User"; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
			if got, want := record.Email, "you@example.com"; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		}
	})

	t.Run("retains_system_admin", func(t *testing.T) {
		t.Parallel()

		admin, _, realm := provisionUsers(t, harness.Database)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":  []string{admin.Name},
			"email": []string{admin.Email},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/realm/users/"; !strings.HasPrefix(got, want) {
			t.Errorf("expected %q to start with %q", got, want)
		}

		records, _, err := realm.ListMemberships(harness.Database, pagination.UnlimitedResults)
		if err != nil {
			t.Fatal(err)
		}

		record, err := harness.Database.FindUser(admin.ID)
		if err != nil {
			t.Fatal(err)
		}

		if record == nil {
			t.Fatalf("failed to find record: %#v", records)
		} else {
			if got, want := record.SystemAdmin, true; got != want {
				t.Errorf("expected %t to be %t", got, want)
			}
		}
	})
}
