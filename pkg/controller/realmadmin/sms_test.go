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
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleSettings_SMS(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	// Create a system configuration.
	if err := harness.Database.SaveSMSConfig(&database.SMSConfig{
		TwilioAccountSid: "sid",
		TwilioAuthToken:  "token",
	}); err != nil {
		t.Fatal(err)
	}

	// Create a system phone number.
	smsFromNumber := &database.SMSFromNumber{
		Label: "Default",
		Value: "+15005550006",
	}
	if err := harness.Database.CreateOrUpdateSMSFromNumbers([]*database.SMSFromNumber{smsFromNumber}); err != nil {
		t.Fatal(err)
	}

	// Mint a cookie for the session.
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
	defer done()

	var twilioAccountSid string
	var twilioAuthToken string
	var twilioFromNumber string

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#sms`),

		// Wait for render.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#twilio-account-sid`, "accountSid", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-auth-token`, "authToken", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number`, "+1234567890", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-sms`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),

		chromedp.Value(`input#twilio-account-sid`, &twilioAccountSid, chromedp.ByQuery),
		chromedp.Value(`input#twilio-auth-token`, &twilioAuthToken, chromedp.ByQuery),
		chromedp.Value(`input#twilio-from-number`, &twilioFromNumber, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Check form
	if got, want := twilioAccountSid, "accountSid"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := twilioAuthToken, project.PasswordSentinel; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}
	if got, want := twilioFromNumber, "+1234567890"; got != want {
		t.Errorf("Expected %q to be %q", got, want)
	}

	{
		// Check database
		smsConfig, err := realm.SMSConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if smsConfig == nil {
			t.Fatal("expected smsConfig")
		}

		if got, want := smsConfig.TwilioAccountSid, "accountSid"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := smsConfig.TwilioAuthToken, "authToken"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := smsConfig.TwilioFromNumber, "+1234567890"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	}

	// Update
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#sms`),

		// Wait for render.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#twilio-account-sid`, "accountSid-new", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number`, "+1987654320", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-sms`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	{
		// Check database again
		smsConfig, err := realm.SMSConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if smsConfig == nil {
			t.Fatal("expected smsConfig")
		}

		if got, want := smsConfig.TwilioAccountSid, "accountSid-new"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := smsConfig.TwilioAuthToken, "authToken"; got != want {
			// should not change
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := smsConfig.TwilioFromNumber, "+1987654320"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	}

	// Delete
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#sms`),

		// Wait for render.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#twilio-account-sid`, "", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-auth-token`, "", chromedp.ByQuery),
		chromedp.SetValue(`input#twilio-from-number`, "", chromedp.ByQuery),

		// Click submit.
		chromedp.Click(`input#update-sms`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Check database again
	if _, err := realm.SMSConfig(harness.Database); !database.IsNotFound(err) {
		t.Fatal("expected smsConfig to be deleted")
	}

	// Update realm to be allowed to use the system config.
	realm.CanUseSystemSMSConfig = true
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Update to use system config
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit page.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/settings#sms`),

		// Wait for render.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.Click(`input#use-system-sms-config`, chromedp.ByQuery),
		chromedp.SendKeys(`select#sms-from-number-id`, `Default`),
		chromedp.SendKeys(`select#sms-country`, `Mexico`),

		// Click submit.
		chromedp.Click(`input#update-sms`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`div#sms`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	{
		realm, err := harness.Database.FindRealm(realm.ID)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := realm.UseSystemSMSConfig, true; got != want {
			t.Errorf("expected %t to be %t", got, want)
		}

		if got, want := realm.SMSFromNumberID, smsFromNumber.ID; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}

		if got, want := realm.SMSCountry, "mx"; got != want {
			t.Errorf("expected %v to be %v", got, want)
		}
	}
}
