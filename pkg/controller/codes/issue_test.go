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

package codes_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleIssue_IssueCode(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

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
	if err := harness.Database.SaveUser(admin, database.SystemTest); err != nil {
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

	yesterday := time.Now().Add(-24 * time.Hour).Format(project.RFC3339Date)

	var code string
	if err := chromedp.Run(taskCtx,
		// Pre-authenticate the user.
		browser.SetCookie(cookie),

		// Visit /codes/issue.
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/issue`),

		// Wait for render.
		chromedp.WaitVisible(`body#codes-issue`, chromedp.ByQuery),

		// Add a date fields
		chromedp.SetValue(`input#test-date`, yesterday, chromedp.ByQuery),
		chromedp.SetValue(`input#symptom-date`, yesterday, chromedp.ByQuery),

		// Click the issue button.
		chromedp.Click(`#submit`, chromedp.ByQuery),
		chromedp.WaitVisible(`#short-code`, chromedp.ByQuery),

		// Get the code.
		chromedp.TextContent(`#short-code`, &code, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	// Verify code length.
	if got, want := len(code), 8; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Verify the code exists.
	dbCode, err := harness.Database.FindVerificationCode(code)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := dbCode.TestType, "confirmed"; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	if got, want := dbCode.Claimed, false; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Exchange the code for a verification certificate.
	allowedTypes := api.AcceptTypes{api.TestTypeConfirmed: struct{}{}}
	token, err := harness.Database.VerifyCodeAndIssueToken(realm.ID, code, allowedTypes, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := token.TestType, api.TestTypeConfirmed; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}
