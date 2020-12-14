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

package user_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"

	"github.com/chromedp/chromedp"
)

func TestHandleSearch(t *testing.T) {
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

	// Create another user.
	user := &database.User{
		Email: "user@example.com",
		Name:  "User",
	}
	if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(harness.Database, realm, rbac.LegacyRealmUser, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	// Create a browser runner.
	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /realm/users.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/realm/users`),

		// Wait for render.
		chromedp.WaitVisible(`body#users-index`, chromedp.ByQuery),

		// Fill out the form.
		chromedp.SetValue(`input#search`, "@example.com", chromedp.ByQuery),
		chromedp.Submit(`form#search-form`, chromedp.ByQuery),

		// Wait for render.
		chromedp.WaitVisible(`body#users-index`, chromedp.ByQuery),

		// Look for the user.
		chromedp.WaitVisible(fmt.Sprintf(`tr#user-%d`, user.ID), chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}
