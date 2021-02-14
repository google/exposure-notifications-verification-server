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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/sessions"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	userpkg "github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleCreate(t *testing.T) {
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

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)
		handler := c.HandleCreate()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
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
		handler := c.HandleCreate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{
			"name":        []string{"person"},
			"email":       []string{"you@example.com"},
			"permissions": []string{"2", "4", "8"},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("expected %s to contain %q", got, want)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)
		handler := c.HandleCreate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.LegacyRealmAdmin,
		})

		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{
			"name": []string{""},
		}.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "cannot be blank"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("creates", func(t *testing.T) {
		t.Parallel()

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
		defer done()

		var apiKey string
		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/apikeys/new`),
			chromedp.WaitVisible(`body#apikeys-new`, chromedp.ByQuery),

			chromedp.SetValue(`input#name`, "Example API key", chromedp.ByQuery),
			chromedp.SetValue(`select#type`, strconv.Itoa(int(database.APIKeyTypeDevice)), chromedp.ByQuery),
			chromedp.Click(`#submit`, chromedp.ByQuery),

			chromedp.WaitVisible(`body#apikeys-show`, chromedp.ByQuery),
			chromedp.Value(`#apikey-value`, &apiKey, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}

		// Ensure API key is valid.
		record, err := harness.Database.FindAuthorizedAppByAPIKey(apiKey)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := record.RealmID, realm.ID; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := record.Name, "Example API key"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
		if got, want := record.APIKeyType, database.APIKeyTypeDevice; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	})
}
