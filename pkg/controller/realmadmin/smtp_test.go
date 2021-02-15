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
	"context"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleSettings_SMTP(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	// Mint a cookie for the session.
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
	defer done()

	var smtpAccount string
	var smtpPassword string
	var smtpHost string

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#email`),

		// Wait for render.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#smtp-account`, "myAccount", chromedp.ByQuery),
		chromedp.SetValue(`input#smtp-password`, "superSecret", chromedp.ByQuery),
		chromedp.SetValue(`input#smtp-host`, "1.1.1.1", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-smtp`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),

		chromedp.Value(`input#smtp-account`, &smtpAccount, chromedp.ByQuery),
		chromedp.Value(`input#smtp-password`, &smtpPassword, chromedp.ByQuery),
		chromedp.Value(`input#smtp-host`, &smtpHost, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Check form
	if got, want := smtpAccount, "myAccount"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := smtpPassword, project.PasswordSentinel; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := smtpHost, "1.1.1.1"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}

	{
		// Check database
		emailConfig, err := realm.EmailConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if emailConfig == nil {
			t.Fatal("expected emailConfig")
		}

		if got, want := emailConfig.SMTPAccount, "myAccount"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := emailConfig.SMTPPassword, "superSecret"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := emailConfig.SMTPHost, "1.1.1.1"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	}

	// Update
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#email`),

		// Wait for render.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#smtp-account`, "myAccount-new", chromedp.ByQuery),
		chromedp.SetValue(`input#smtp-host`, "1.1.1.1-new", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-smtp`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	{
		// Check database again
		emailConfig, err := realm.EmailConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if emailConfig == nil {
			t.Fatal("expected emailConfig")
		}

		if got, want := emailConfig.SMTPAccount, "myAccount-new"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := emailConfig.SMTPPassword, "superSecret"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := emailConfig.SMTPHost, "1.1.1.1-new"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	}

	// Delete
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#email`),

		// Wait for render.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#smtp-account`, "", chromedp.ByQuery),
		chromedp.SetValue(`input#smtp-password`, "", chromedp.ByQuery),
		chromedp.SetValue(`input#smtp-host`, "", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-smtp`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#email`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Check database again
	if _, err := realm.EmailConfig(harness.Database); !database.IsNotFound(err) {
		t.Fatal("expected emailConfig to be deleted")
	}
}
