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

package smskeys_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/smskeys"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandleActivate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, harness.Config.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		t.Fatal(err)
	}

	c := smskeys.New(harness.Config, harness.Database, publicKeyCache, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleActivate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := smskeys.New(harness.Config, harness.BadDatabase, publicKeyCache, harness.Renderer)
		handler := c.HandleActivate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "", &url.Values{})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       &database.Realm{},
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "", &url.Values{
			"id": []string{"123456789"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Body.String(), "does not exist"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := realm.CreateSMSSigningKeyVersion(ctx, harness.Database, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		list, err := realm.ListSMSSigningKeys(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) < 1 {
			t.Fatalf("expected at least one key")
		}
		signingKey := list[0]

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "", &url.Values{
			"id": []string{fmt.Sprintf("%d", signingKey.ID)},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), "/realm/sms-keys"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}

		// Check key was marked active
		key, err := harness.Database.FindSMSSigningKey(signingKey.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := key.Active, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
	})
}
