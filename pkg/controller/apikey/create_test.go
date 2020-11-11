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

package apikey_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleCreate(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t)

	// Get the default realm
	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	// Create a user
	admin := &database.User{
		Email:       "admin@example.com",
		Name:        "Admin",
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
	taskCtx, done := context.WithTimeout(browserCtx, 60*time.Second)
	defer done()

	var apiKey string
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /apikeys/new.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/apikeys/new`),

		// Wait for render.
		chromedp.WaitVisible(`body#apikeys-new`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#name`, "Example API key", chromedp.ByQuery),
		chromedp.SetValue(`select#type`, strconv.Itoa(int(database.APIKeyTypeDevice)), chromedp.ByQuery),

		// Click the submit button.
		chromedp.Click(`#submit`, chromedp.ByQuery),

		// Wait for the page to reload.
		chromedp.WaitVisible(`body#apikeys-show`, chromedp.ByQuery),

		// Get the API key.
		chromedp.Value(`#apikey-value`, &apiKey, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Ensure API key is valid.
	record, err := harness.Database.FindAuthorizedAppByAPIKey(apiKey)
	if err != nil {
		t.Fatal(err)
	}

	// Verify name.
	if got, want := record.Name, "Example API key"; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Verify API key type.
	if got, want := record.APIKeyType, database.APIKeyTypeDevice; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}
