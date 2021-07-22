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

package mobileapps_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/mobileapps"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandleCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := mobileapps.New(harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleCreate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := mobileapps.New(harness.BadDatabase, harness.Renderer)
		handler := c.HandleCreate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":   []string{"banana"},
			"url":    []string{"http://example.com"},
			"os":     []string{"1"},
			"app_id": []string{"com.example.app"},
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
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{})
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

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"name":   []string{"Example mobile app"},
			"url":    []string{"https://example.com"},
			"os":     []string{strconv.Itoa(int(database.OSTypeIOS))},
			"app_id": []string{"com.example.app"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), "/realm/mobile-apps/1"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		record, err := realm.FindMobileApp(harness.Database, 1)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := record.RealmID, realm.ID; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := record.Name, "Example mobile app"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := record.URL, "https://example.com"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})
}
