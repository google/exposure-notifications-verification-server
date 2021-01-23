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
	"net/http/httptest"
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

func TestHandleCreate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
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
		handler := c.HandleCreateKey()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleCreateKey()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

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

	t.Run("creates", func(t *testing.T) {
		t.Parallel()

		c := smskeys.New(cfg, harness.Database, publicKeyCache, h)
		handler := c.HandleCreateKey()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/realm/sms-keys"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		updatedRealm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		keys, err := updatedRealm.ListSMSSigningKeys(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) != 1 {
			t.Fatalf("no SMS key present after create")
		}
	})
}
