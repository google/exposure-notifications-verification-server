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

package realmadmin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandleSettings_Email(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, harness.Renderer, harness.Cacher)
	handler := middleware.InjectCurrentPath()(c.HandleSettings())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		wantSMTPAccount := "my-account"
		wantSMTPPassword := "my-password"
		wantSMTPHost := "example.com"
		wantSMTPPort := "123"

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"email":         []string{"1"},
			"smtp_account":  []string{wantSMTPAccount},
			"smtp_password": []string{wantSMTPPassword},
			"smtp_host":     []string{wantSMTPHost},
			"smtp_port":     []string{wantSMTPPort},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}

		emailConfig, err := realm.EmailConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if emailConfig.SMTPAccount != wantSMTPAccount {
			t.Errorf("expected %q to be %q", emailConfig.SMTPAccount, wantSMTPAccount)
		}
		if emailConfig.SMTPPassword != wantSMTPPassword {
			t.Errorf("expected %q to be %q", emailConfig.SMTPPassword, wantSMTPPassword)
		}
		if emailConfig.SMTPHost != wantSMTPHost {
			t.Errorf("expected %q to be %q", emailConfig.SMTPHost, wantSMTPHost)
		}
		if emailConfig.SMTPPort != wantSMTPPort {
			t.Errorf("expected %q to be %q", emailConfig.SMTPPort, wantSMTPPort)
		}
	})
}
