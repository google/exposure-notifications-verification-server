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
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmkeys"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/keyutils"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestRealmKeys_SubmitActivate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithSession(ctx, session)

	cfg := &config.ServerConfig{}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	publicKeyCache, err := keyutils.NewPublicKeyCache(ctx, harness.Cacher, cfg.CertificateSigning.PublicKeyCacheDuration)
	if err != nil {
		t.Fatal(err)
	}
	c := realmkeys.New(cfg, harness.Database, harness.KeyManager, publicKeyCache, h)
	handler := c.HandleActivate()

	envstest.ExerciseSessionMissing(t, handler)
	envstest.ExerciseMembershipMissing(t, handler)
	envstest.ExercisePermissionMissing(t, handler)

	ctx = controller.WithMembership(ctx, &database.Membership{
		User:        user,
		Realm:       realm,
		Permissions: rbac.SettingsWrite,
	})

	// no form bound
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

		// shows original page with error flash
		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", result.StatusCode)
		}
	}()

	// no key exists
	func() {
		form := url.Values{}
		form.Add("id", "1")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		// shows original page with error flash
		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", result.StatusCode)
		}
	}()

	if _, err := realm.CreateSigningKeyVersion(ctx, harness.Database, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	list, err := realm.ListSigningKeys(harness.Database)
	if err != nil {
		t.Fatal(err)
	}

	// success
	func() {
		form := url.Values{}
		form.Add("id", fmt.Sprint(list[0].ID))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(form.Encode()))
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
