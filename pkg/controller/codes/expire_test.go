// Copyright 2021 Google LLC
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
	"github.com/google/exposure-notifications-verification-server/internal/i18n"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleExpire_ExpireCode(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		RealmID: realm.ID,
		Name:    "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	c := codes.NewServer(harness.Config, harness.Database, harness.Renderer)
	handler := middleware.ProcessLocale(locales)(c.HandleExpirePage())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := codes.NewServer(harness.Config, harness.BadDatabase, harness.Renderer)
		handler := middleware.ProcessLocale(locales)(c.HandleExpirePage())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.CodeExpire | rbac.CodeRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"uuid": "aaa-bbb-ccc-ddd"})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		code := &database.VerificationCode{
			RealmID:       realm.ID,
			Code:          "00000001",
			LongCode:      "00000001ABC",
			Claimed:       false,
			TestType:      "confirmed",
			ExpiresAt:     time.Now().Add(time.Hour),
			LongExpiresAt: time.Now().Add(time.Hour),
		}
		if err := harness.Database.SaveVerificationCode(code, realm); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.CodeExpire | rbac.CodeRead,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"uuid": code.UUID})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		record, err := realm.FindVerificationCodeByUUID(harness.Database, code.UUID)
		if err != nil {
			t.Fatal(err)
		}
		if now := time.Now().UTC(); record.ExpiresAt.After(now) {
			t.Errorf("expected code expired. got %s but now is %s", code.ExpiresAt, now)
		}
	})
}

func TestHandleExpireAPI_ExpireCode(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		RealmID: realm.ID,
		Name:    "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	locales, err := i18n.Load(harness.Config.LocalesPath)
	if err != nil {
		t.Fatal(err)
	}

	c := codes.NewServer(harness.Config, harness.Database, harness.Renderer)
	handler := middleware.ProcessLocale(locales)(c.HandleExpireAPI())

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithAuthorizedApp(ctx, nil)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", &api.ExpireCodeRequest{
			UUID: "123e4567-e89b-12d3-a456-426614174000",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithAuthorizedApp(ctx, authApp)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", &api.ExpireCodeRequest{
			UUID: "123e4567-e89b-12d3-a456-426614174000",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusNotFound; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("bad_uuid", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithAuthorizedApp(ctx, authApp)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", &api.ExpireCodeRequest{
			UUID: "aaa-bbb-ccc",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusNotFound; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("bad_request", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithAuthorizedApp(ctx, authApp)

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", map[string]string{
			"hello": "world",
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusBadRequest; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithAuthorizedApp(ctx, authApp)

		code := &database.VerificationCode{
			RealmID:       realm.ID,
			Code:          "00000001",
			LongCode:      "00000001ABC",
			Claimed:       false,
			TestType:      "confirmed",
			ExpiresAt:     time.Now().Add(time.Hour),
			LongExpiresAt: time.Now().Add(time.Hour),
			IssuingAppID:  authApp.ID,
		}
		if err := harness.Database.SaveVerificationCode(code, realm); err != nil {
			t.Fatal(err)
		}

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", &api.ExpireCodeRequest{
			UUID: code.UUID,
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		record, err := realm.FindVerificationCodeByUUID(harness.Database, code.UUID)
		if err != nil {
			t.Fatal(err)
		}
		if now := time.Now().UTC(); record.ExpiresAt.After(now) {
			t.Errorf("expected code expired. got %s but now is %s", code.ExpiresAt, now)
		}
	})
}
