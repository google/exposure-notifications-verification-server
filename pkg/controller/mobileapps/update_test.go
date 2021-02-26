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

package mobileapps_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/mobileapps"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	app := &database.MobileApp{
		RealmID: realm.ID,
		Name:    "Appy",
		AppID:   "com.example.app",
		URL:     "https://app.example.com",
		OS:      database.OSTypeIOS,
	}
	if err := harness.Database.SaveMobileApp(app, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	c := mobileapps.New(harness.Database, harness.Renderer)
	handler := c.HandleUpdate()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.MobileAppWrite,
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := mobileapps.New(harness.BadDatabase, harness.Renderer)
		handler := c.HandleUpdate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name": []string{"apple"},
		})
		r = mux.SetURLVars(r, map[string]string{"id": "1"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name": []string{""},
			"os":   []string{"-1"},
		})
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", app.ID)})
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

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.MobileAppWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"name":            []string{"Updated name"},
			"app_id":          []string{"com.updated.example.app"},
			"os":              []string{fmt.Sprintf("%d", app.OS)},
			"enable_redirect": []string{"false"},
		})
		r = mux.SetURLVars(r, map[string]string{"id": fmt.Sprintf("%d", app.ID)})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), fmt.Sprintf("/realm/mobile-apps/%d", app.ID); got != want {
			t.Errorf("expected %s to be %s", got, want)
		}

		record, err := realm.FindMobileApp(harness.Database, app.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := record.Name, "Updated name"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := record.AppID, "com.updated.example.app"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := record.DisableRedirect, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
	})
}
