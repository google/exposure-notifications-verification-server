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

package home_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleHome_IssueCode(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	admin := &database.User{
		Email:       "admin@example.com",
		Name:        "Admin",
		Realms:      []*database.Realm{realm},
		AdminRealms: []*database.Realm{realm},
	}
	if err := harness.Database.SaveUser(admin, database.System); err != nil {
		t.Fatal(err)
	}

	browserCtx := browser.New(t)

	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	var code string
	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(`http://`+harness.Server.Addr()),
		Login("admin@example.com", "Password"),

		// Post-login action is /home.
		chromedp.WaitVisible(`body#home`, chromedp.ByQuery),

		// Click the issue button.
		chromedp.Click(`#submit`, chromedp.ByQuery),
		chromedp.WaitVisible(`#code`, chromedp.ByQuery),

		// Get the code
		chromedp.TextContent(`#code`, &code, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	if got, want := len(code), 8; got != want {
		t.Errorf("expected %v to be %v", got, want)
	}
}

// Login executes a login for the given user.
// TODO(sethvargo): move to global helper.
func Login(email, password string) chromedp.Action {
	return chromedp.Tasks{
		chromedp.WaitVisible(`body#login`, chromedp.ByQuery),
		chromedp.SendKeys(`#email`, email, chromedp.NodeReady, chromedp.ByQuery),
		chromedp.SendKeys(`#password`, password, chromedp.NodeReady, chromedp.ByQuery),
		chromedp.Click(`#login-form #submit`, chromedp.NodeReady, chromedp.ByQuery),

		// Skip mobile phone verification for tests.
		chromedp.WaitVisible(`body#login-register-phone`, chromedp.ByQuery),
		chromedp.Click(`#skip`, chromedp.NodeReady, chromedp.ByQuery),
	}
}
