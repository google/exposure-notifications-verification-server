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

package login_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleVerifyEmailSend_ShowVerifyEmail(t *testing.T) {
	t.Parallel()

	harness := envstest.NewServer(t, testDatabaseInstance)

	_, _, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	cookie, err := harness.SessionCookie(session)
	if err != nil {
		t.Fatal(err)
	}

	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, 30*time.Second)
	defer done()

	if err := chromedp.Run(taskCtx,
		browser.SetCookie(cookie),
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/login/manage-account?mode=verifyEmail`),
		chromedp.WaitVisible(`body#verify-email`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}

func TestHandleVerifyEmailSend_SubmitVerifyEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, user, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithSession(ctx, session)

	emailConfig := &database.EmailConfig{
		RealmID:      realm.ID,
		ProviderType: email.ProviderTypeNoop,
		SMTPAccount:  "noreply@sendemails.meh",
		SMTPPassword: "my-secret-ref",
		SMTPHost:     "smtp.sendemails.meh",
	}
	if err := harness.Database.SaveEmailConfig(emailConfig); err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, h)
	handler := c.HandleSubmitVerifyEmail()

	envstest.ExerciseSessionMissing(t, handler)
	envstest.ExerciseMembershipMissing(t, handler)

	// success
	func() {
		ctx = controller.WithMembership(ctx, &database.Membership{
			User:  user,
			Realm: realm,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()
	}()
}
