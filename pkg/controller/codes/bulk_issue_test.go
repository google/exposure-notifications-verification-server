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

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestRenderBulkIssue(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	realm.AllowBulkUpload = true
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	if err := user.AddToRealm(harness.Database, realm,
		rbac.LegacyRealmAdmin|rbac.CodeBulkIssue, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	sms := &database.SMSConfig{
		RealmID:          realm.ID,
		ProviderType:     sms.ProviderType("TWILIO"),
		TwilioAccountSid: "abc123",
		TwilioAuthToken:  "def123",
		TwilioFromNumber: "+11234567890",
	}
	if err := harness.Database.SaveSMSConfig(sms); err != nil {
		t.Fatalf("failed to save SMSConfig: %v", err)
	}

	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	if err := chromedp.Run(taskCtx,
		browser.SetCookie(cookie),
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/bulk-issue`),
		chromedp.WaitVisible(`body#bulk-issue`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}
