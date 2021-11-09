// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleIssue_IssueCode(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	realm.WelcomeMessage = "Welcome Test"
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		RealmID: realm.ID,
		Name:    "Appy",
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	var code string
	var html string
	var lastErr error

	// This test has a tendency to be flakey because we rely on downloading
	// upstream javascript and styles. Sometimes the upstream CDN blocks the
	// download, which causes the test to fail. We retry 3 times to try and
	// mitigate this.
	for i := 0; i < 3; i++ {
		if err := func() error {
			browserCtx := browser.New(t)
			taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
			defer done()

			yesterday := time.Now().Add(-24 * time.Hour).Format(project.RFC3339Date)

			if err := chromedp.Run(taskCtx,
				browser.SetCookie(cookie),
				chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/issue`),
				chromedp.OuterHTML("html", &html, chromedp.ByQuery),
				chromedp.WaitVisible(`body#codes-issue`, chromedp.ByQuery),
				chromedp.OuterHTML("html", &html, chromedp.ByQuery),

				chromedp.SetValue(`input#test-date`, yesterday, chromedp.ByQuery),
				chromedp.SetValue(`input#symptom-date`, yesterday, chromedp.ByQuery),
				chromedp.OuterHTML("html", &html, chromedp.ByQuery),
				chromedp.WaitVisible(`#submit`, chromedp.ByQuery),
				chromedp.Click(`#submit`, chromedp.ByQuery),

				chromedp.OuterHTML("html", &html, chromedp.ByQuery),
				chromedp.WaitVisible(`#short-code`, chromedp.ByQuery),
				chromedp.TextContent(`#short-code`, &code, chromedp.ByQuery),
				chromedp.OuterHTML("html", &html, chromedp.ByQuery),
			); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			lastErr = err
			continue
		} else {
			lastErr = nil
			break
		}
	}

	if lastErr != nil {
		t.Fatalf("failed to issue code: %s\nlast html:\n\n%s", lastErr, html)
	}

	// Verify code length.
	if got, want := len(code), 8; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Verify the code exists.
	dbCode, err := realm.FindVerificationCode(harness.Database, code)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := dbCode.TestType, "confirmed"; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	if got, want := dbCode.Claimed, false; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	if got, want := dbCode.IssuingUserID, user.ID; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}

	// Exchange the code for a verification certificate.
	allowedTypes := api.AcceptTypes{api.TestTypeConfirmed: struct{}{}}
	request := &database.IssueTokenRequest{
		Time:        now,
		AuthApp:     authApp,
		VerCode:     code,
		AcceptTypes: allowedTypes,
		ExpireAfter: 30 * time.Minute,
	}
	token, err := harness.Database.VerifyCodeAndIssueToken(request)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := token.TestType, api.TestTypeConfirmed; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}
