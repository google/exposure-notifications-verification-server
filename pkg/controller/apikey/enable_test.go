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

package apikey_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/envstest/testconfig"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/apikey"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
)

func TestHandleEnable(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(context.Background(), testconfig.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}
		c := apikey.New(harness.Cacher, harness.Database, h)
		handler := c.HandleEnable()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		h, err := render.New(context.Background(), testconfig.ServerAssetsPath(), true)
		if err != nil {
			t.Fatal(err)
		}
		c := apikey.New(harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/{id}", c.HandleEnable())

		ctx := context.Background()
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.APIKeyWrite,
		})

		r := httptest.NewRequest("GET", "/100", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "unauthorized"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("disables", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC().Add(-5 * time.Second)
		authApp := &database.AuthorizedApp{
			RealmID: realm.ID,
			Name:    "Disables app",
			Model: gorm.Model{
				DeletedAt: &now,
			},
		}
		if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, 10*time.Second)
		defer done()

		// Click "confirm" when it pops up.
		confirmErrCh := testconfig.AutoConfirmDialogs(taskCtx, true)

		if err := chromedp.Run(taskCtx,
			browser.SetCookie(cookie),
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/apikeys`),
			chromedp.WaitVisible(`body#apikeys-index`, chromedp.ByQuery),

			chromedp.Click(fmt.Sprintf(`a#enable-apikey-%d`, authApp.ID), chromedp.ByQuery),

			chromedp.WaitVisible(`body#apikeys-index`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
		}

		if err := <-confirmErrCh; err != nil {
			t.Fatal(err)
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
