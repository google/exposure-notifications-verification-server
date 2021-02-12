// Copyright 2021 Google LLC
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
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"

	"github.com/chromedp/chromedp"
)

func TestHandleShow_ShowCodeStatus(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
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

	vc := &database.VerificationCode{
		RealmID:       realm.ID,
		Code:          "00000001",
		LongCode:      "00000001ABC",
		Claimed:       true,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
	}
	if err := harness.Database.SaveVerificationCode(vc, realm); err != nil {
		t.Fatal(err)
	}

	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 120*time.Second)
	defer done()

	if err := chromedp.Run(taskCtx,
		browser.SetCookie(cookie),
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/`+vc.UUID),
		chromedp.WaitVisible(`body#codes-show`, chromedp.ByQuery),
		chromedp.WaitNotPresent(`body#code-expire`, chromedp.ByQuery),

		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/invalidcode`),
		chromedp.WaitVisible(`body#codes-index`, chromedp.ByQuery), // redirect to index

		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/`+strings.ToUpper(vc.UUID)),
		chromedp.WaitVisible(`body#codes-show`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	if code, err := realm.FindVerificationCodeByUUID(harness.Database, vc.UUID); err != nil {
		t.Fatal(err)
	} else if code.ExpiresAt.Before(now) {
		t.Errorf("expected code not expired. got %s but now is %s", code.ExpiresAt, now)
	}
}
