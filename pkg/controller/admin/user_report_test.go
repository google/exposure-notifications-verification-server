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

package admin_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/sessions"
)

func TestHandleUserReportPurge(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleUserReportPurge())

	user, err := harness.Database.FindUser(1)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseUserMissing(t, handler)
	})

	t.Run("invalid_phone", func(t *testing.T) {
		t.Parallel()

		session := &sessions.Session{}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleUserReportPurge())

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"phone_number[full]": []string{"nope"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("Expected %d to be %d: \n%s\n", got, want, w.Body.String())
		}

		flash := controller.Flash(session)
		errs := flash.Errors()
		if got, want := len(errs), 1; got != want {
			t.Errorf("Expected %d errors, got %d", want, got)
		}
		if got, want := errs[0], "Failed to decode phone number"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"phone_number[full]": []string{"+11234567890"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d: \n%s\n", got, want, w.Body.String())
		}
	})

	t.Run("deletes", func(t *testing.T) {
		t.Parallel()

		phoneNumber := "+12065551234"
		userReport, err := harness.Database.NewUserReport(phoneNumber, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := harness.Database.RawDB().Save(userReport).Error; err != nil {
			t.Fatal(err, userReport.ErrorMessages())
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)

		c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleUserReportPurge())

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"phone_number[full]": []string{phoneNumber},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d: \n%s\n", got, want, w.Body.String())
		}

		if _, err := harness.Database.FindUserReport(phoneNumber); !database.IsNotFound(err) {
			t.Errorf("expected not found, got %#v", err)
		}
	})
}
