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

package realmkeys_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmkeys"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestHandleManualRotate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.ServerConfig{}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, h)
		handler := c.HandleManualRotate()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("not_enabled", func(t *testing.T) {
		t.Parallel()

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, h)
		handler := c.HandleManualRotate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AutoRotateCertificateKey: false,
			},
			User:        &database.User{},
			Permissions: rbac.SettingsWrite,
		})

		r := httptest.NewRequest("PUT", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 422; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Already in manual key rotation mode"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, h)
		handler := c.HandleManualRotate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AutoRotateCertificateKey: true,
			},
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
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("enables", func(t *testing.T) {
		t.Parallel()

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, h)
		handler := c.HandleManualRotate()

		realm := database.NewRealmWithDefaults("test")
		realm.AutoRotateCertificateKey = true
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

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
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Header().Get("Location"), "/realm/keys"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}

		updatedRealm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := updatedRealm.AutoRotateCertificateKey, false; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}
	})
}
