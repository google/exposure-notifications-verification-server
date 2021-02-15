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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	userpkg "github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/chromedp/chromedp"
)

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, admin, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	// Create another user.
	user := &database.User{
		Email: "user@example.com",
		Name:  "User",
	}
	if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(harness.Database, realm, rbac.LegacyRealmAdmin, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)
		handler := c.HandleUpdate()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
		envstest.ExerciseIDNotFound(t, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.UserWrite,
		}, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/{id}", c.HandleUpdate()).Methods(http.MethodPut)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		u := fmt.Sprintf("/%d", user.ID)
		r := httptest.NewRequest(http.MethodPut, u, strings.NewReader(url.Values{
			"name": []string{"apple"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/{id}", c.HandleUpdate()).Methods(http.MethodPut)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		u := fmt.Sprintf("/%d", user.ID)
		r := httptest.NewRequest(http.MethodPut, u, strings.NewReader(url.Values{
			"name": []string{""},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
		defer done()

		u := fmt.Sprintf(`http://`+harness.Server.Addr()+`/realm/users/%d/edit`, user.ID)

		for _, permission := range rbac.NamePermissionMap {
			permission := permission
			targets := []string{fmt.Sprintf(`input#permission-%s`, permission)}

			// We also need to remove permissions that imply this permission, or it
			// will be added back in.
			for _, superPerm := range rbac.ImpliedBy(permission) {
				targets = append(targets, fmt.Sprintf(`input#permission-%s`, superPerm))
			}

			// Build the actions as an array prior to run since super-permissions
			// actions aren't known until runtime.
			actions := []chromedp.Action{
				browser.SetCookie(cookie),
				chromedp.Navigate(u),
				chromedp.WaitVisible(`body#users-edit`, chromedp.ByQuery),
			}

			// Fill out the form.
			for _, target := range targets {
				actions = append(actions, chromedp.RemoveAttribute(target, "checked", chromedp.ByQuery))
			}
			actions = append(actions, chromedp.Submit(`form#user-form`, chromedp.ByQuery))

			actions = append(actions, chromedp.WaitVisible(`body#users-show`, chromedp.ByQuery))

			if err := chromedp.Run(taskCtx, actions...); err != nil {
				t.Fatal(err)
			}
		}

		// Assert the user has no permissions left
		membership, err := user.FindMembership(harness.Database, realm.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int64(membership.Permissions), int64(0); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}

		// Now add permissions back
		for _, permission := range rbac.NamePermissionMap {
			permission := permission
			target := fmt.Sprintf(`input#permission-%s`, permission)

			if err := chromedp.Run(taskCtx,
				browser.SetCookie(cookie),
				chromedp.Navigate(u),
				chromedp.WaitVisible(`body#users-edit`, chromedp.ByQuery),

				chromedp.SetAttributeValue(target, "checked", "true", chromedp.ByQuery),
				chromedp.Submit(`form#user-form`, chromedp.ByQuery),

				chromedp.WaitVisible(`body#users-show`, chromedp.ByQuery),
			); err != nil {
				t.Fatal(err)
			}
		}

		// Permissions should be back
		membership, err = user.FindMembership(harness.Database, realm.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int64(membership.Permissions), int64(32766); got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})
}
