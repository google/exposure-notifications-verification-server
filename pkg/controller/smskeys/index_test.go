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
	"net/http"
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
	"github.com/gorilla/sessions"
)

func TestHandleIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	cfg := &config.ServerConfig{}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := smskeys.New(cfg, harness.Database, publicKeyCache, harness.Renderer)
		handler := c.HandleIndex()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("no_keys", func(t *testing.T) {
		t.Parallel()

		publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
		if err != nil {
			t.Fatal(err)
		}
		c := smskeys.New(cfg, harness.Database, publicKeyCache, harness.Renderer)
		handler := c.HandleIndex()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		if !strings.Contains(w.Body.String(), "There are no SMS signing keys active for this realm.") {
			t.Fatalf("missing required text")
		}
	})
}
