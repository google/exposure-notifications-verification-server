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

	"github.com/chromedp/chromedp"
)

func TestShowAdminSMS(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	// Get the default realm
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Get the system admin
	admin, err := harness.Database.FindUser(1)
	if err != nil {
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
	taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
	defer done()

	wantAccountSid := "abc123"
	wantAuthToken := "def456"
	wantFromNumber1 := "+11234567890"
	wantFromNumber2 := "+99999999999"

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /admin
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/sms`),

		// Wait for render
		chromedp.WaitVisible(`body#admin-sms-show`, chromedp.ByQuery),

		// Set fields
		chromedp.SetValue(`input#twilio-account-sid`, wantAccountSid, chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-auth-token`, wantAuthToken, chromedp.ByQuery),

		// From number 1
		chromedp.Click(`a#add-phone-number`, chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number-0-label`, "aaa", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number-0-value`, wantFromNumber1, chromedp.ByQuery),

		// From number 2
		chromedp.Click(`a#add-phone-number`, chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number-1-label`, "zzz", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number-1-value`, wantFromNumber2, chromedp.ByQuery),

		// Submit form
		chromedp.Submit(`form#sms-form`, chromedp.ByQuery),

		// Wait for render.
		chromedp.WaitVisible(`body#admin-sms-show`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	systemSMSConfig, err := harness.Database.SystemSMSConfig()
	if err != nil {
		t.Fatal(err)
	}

	if systemSMSConfig.TwilioAccountSid != wantAccountSid {
		t.Errorf("got: %s, want: %s", systemSMSConfig.TwilioAccountSid, wantAccountSid)
	}
	if systemSMSConfig.TwilioAuthToken != wantAuthToken {
		t.Errorf("got: %s, want: %s", systemSMSConfig.TwilioAuthToken, wantAuthToken)
	}

	smsFromNumbers, err := harness.Database.SMSFromNumbers()
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(smsFromNumbers), 2; got != want {
		t.Fatalf("expected %d to be %d", got, want)
	}

	aaa := smsFromNumbers[0]
	if got, want := aaa.Label, "aaa"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := aaa.Value, wantFromNumber1; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}

	zzz := smsFromNumbers[1]
	if got, want := zzz.Label, "zzz"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := zzz.Value, wantFromNumber2; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
}
