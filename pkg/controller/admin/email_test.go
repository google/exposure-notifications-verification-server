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

package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/gorilla/sessions"
)

func TestAdminEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := c.HandleEmailUpdate()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := c.HandleEmailUpdate()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, w.Header())
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		wantSMTPAccount := "test=smtp-account"
		wantSMTPPassword := "test-password"
		wantSMTPHost := "smtp.test.example.com"
		wantSMTPPort := "587"

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", &url.Values{
			"smtp_account":  []string{wantSMTPAccount},
			"smtp_password": []string{wantSMTPPassword},
			"smtp_host":     []string{wantSMTPHost},
			"smtp_port":     []string{wantSMTPPort},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}

		systemEmailConfig, err := harness.Database.SystemEmailConfig()
		if err != nil {
			t.Fatal(err)
		}

		if systemEmailConfig.SMTPAccount != wantSMTPAccount {
			t.Errorf("got: %s, want: %s", systemEmailConfig.SMTPAccount, wantSMTPAccount)
		}
		if systemEmailConfig.SMTPPassword != wantSMTPPassword {
			t.Errorf("got: %s, want: %s", systemEmailConfig.SMTPPassword, wantSMTPPassword)
		}
		if systemEmailConfig.SMTPHost != wantSMTPHost {
			t.Errorf("got: %s, want: %s", systemEmailConfig.SMTPHost, wantSMTPHost)
		}
		if systemEmailConfig.SMTPPort != wantSMTPPort {
			t.Errorf("got: %s, want: %s", systemEmailConfig.SMTPPort, wantSMTPPort)
		}
	})
}
