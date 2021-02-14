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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/smskeys"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestHandleActivate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.ServerConfig{}

	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleActivate()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleActivate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		u := &url.Values{"id": []string{"123456"}}

		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(u.Encode()))
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
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleActivate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		u := &url.Values{"id": []string{"123456"}}

		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(u.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "does not exist"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("activates", func(t *testing.T) {
		t.Parallel()

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

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleActivate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        user,
			Permissions: rbac.SettingsWrite,
		})

		u := &url.Values{"id": []string{fmt.Sprintf("%d", signingKey.ID)}}

		r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(u.Encode()))
		r = r.Clone(ctx)
		r.Header.Set("Accept", "text/html")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/realm/sms-keys"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
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
