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

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func TestShowAdminRealms(t *testing.T) {
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

	// This accepts "are you sure" alert that pops up for "leave realm"
	chromedp.ListenTarget(taskCtx, func(ev interface{}) {
		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go func() {
				if err := chromedp.Run(taskCtx,
					page.HandleJavaScriptDialog(true),
				); err != nil {
					panic(err)
				}
			}()
		}
	})

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /admin/realms
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/admin/realms`),

		// Wait for render.
		chromedp.WaitVisible(`body#admin-realms-index`, chromedp.ByQuery),

		/* ----- Test New Realm -----  */
		chromedp.Click(`a#new`, chromedp.ByQuery),
		// Fill out the form.
		chromedp.SetValue(`input#name`, "Test Realm", chromedp.ByQuery),
		chromedp.SetValue(`input#regionCode`, "us-tst", chromedp.ByQuery),
		chromedp.SetValue(`input#certificateIssuer`, "test issuer", chromedp.ByQuery),
		chromedp.SetValue(`input#certificateAudience`, "test audience", chromedp.ByQuery),
		chromedp.Submit(`form#new-form`, chromedp.ByQuery),

		/* ----- Test Search -----  */
		// Wait for render.
		chromedp.WaitVisible(`body#admin-realms-index`, chromedp.ByQuery),

		// Fill out the form with a non-existing realm
		chromedp.SetValue(`input#search`, "notexists", chromedp.ByQuery),
		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

		// Assert no realms shown
		chromedp.WaitNotPresent(`table#results-table tr`, chromedp.ByQuery),

		// Fill out the form by realm name.
		chromedp.SetValue(`input#search`, " test realm ", chromedp.ByQuery),
		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

		// Wait for the search result.
		chromedp.WaitVisible(`table#results-table tr`, chromedp.ByQuery),

		/* ----- Test Edit Realm -----  */
		// Visit the realm from the search
		chromedp.Click(`table#results-table tr td a`, chromedp.ByQuery),

		// Leave the realm
		chromedp.Click(`a#leave`, chromedp.ByQuery),

		// Wait for render.
		chromedp.WaitVisible(`a#join`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}
