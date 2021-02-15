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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestAdminEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	cfg := &config.ServerConfig{}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	c := admin.New(cfg, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()
		envstest.ExerciseSessionMissing(t, c.HandleEmailUpdate())
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := admin.New(cfg, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, h)

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		r := httptest.NewRequest(http.MethodPut, "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		c.HandleEmailUpdate().ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, w.Header())
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		_, _, session, err := harness.ProvisionAndLogin()
		if err != nil {
			t.Fatal(err)
		}

		cookie, err := harness.SessionCookie(session)
		if err != nil {
			t.Fatal(err)
		}

		// Create a browser runner.
		browserCtx := browser.New(t)
		taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
		defer done()

		wantSMTPAccount := "test=smtp-account"
		wantSMTPPassword := "test-password"
		wantSMTPHost := "smtp.test.example.com"
		wantSMTPPort := "587"

		if err := chromedp.Run(taskCtx,
			// Pre-authenticate the user.
			browser.SetCookie(cookie),

			// Visit /admin
			chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/email`),

			// Wait for render.
			chromedp.WaitVisible(`body#admin-email-show`, chromedp.ByQuery),

			// Set fields and submit
			chromedp.SetValue(`input#smtp-account`, wantSMTPAccount, chromedp.ByQuery),
			chromedp.SetValue(`input#smtp-password`, wantSMTPPassword, chromedp.ByQuery),
			chromedp.SetValue(`input#smtp-host`, wantSMTPHost, chromedp.ByQuery),
			chromedp.SetValue(`input#smtp-port`, wantSMTPPort, chromedp.ByQuery),
			chromedp.Submit(`form#email-form`, chromedp.ByQuery),

			// Wait for render.
			chromedp.WaitVisible(`body#admin-email-show`, chromedp.ByQuery),
		); err != nil {
			t.Fatal(err)
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
