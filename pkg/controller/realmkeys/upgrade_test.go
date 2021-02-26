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
	"net/http"
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
)

func TestRealmKeys_SubmitUpgrade(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithSession(ctx, session)

	cfg := &config.ServerConfig{}

	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		t.Fatal(err)
	}
	c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, harness.Renderer)
	handler := c.HandleUpgrade()

	envstest.ExerciseSessionMissing(t, handler)
	envstest.ExerciseMembershipMissing(t, handler)
	envstest.ExercisePermissionMissing(t, handler)

	ctx = controller.WithMembership(ctx, &database.Membership{
		User:        user,
		Realm:       realm,
		Permissions: rbac.SettingsWrite,
	})

	// success
	func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 301 SeeOther, got %d", result.StatusCode)
		}
	}()

	// success - use realm certificate
	func() {
		realm.CertificateIssuer = "foo"
		realm.CertificateAudience = "bar"

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 301 SeeOther, got %d", result.StatusCode)
		}
	}()
}
