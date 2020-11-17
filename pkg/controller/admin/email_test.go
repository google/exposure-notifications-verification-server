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
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestShowAdminEmail(t *testing.T) {
	harness := envstest.NewServer(t)

	// Get the default realm
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Create a system admin
	admin := &database.User{
		Email:       "admin@example.com",
		Name:        "Admin",
		SystemAdmin: true,
		Realms:      []*database.Realm{realm},
		AdminRealms: []*database.Realm{realm},
	}
	if err := harness.Database.SaveUser(admin, database.System); err != nil {
		t.Fatal(err)
	}

	// Log in the user.
	session, err := harness.LoggedInSession(nil, admin.Email)
	if err != nil {
		t.Fatal(err)
	}

	// Set the current realm.
	controller.StoreSessionRealm(session, realm)

	// Mint a cookie for the session.
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}
	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
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
}
