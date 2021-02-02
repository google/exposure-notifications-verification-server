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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
)

func TestHandleResetPassword_ShowResetPassword(t *testing.T) {
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
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/login/reset-password`),
		chromedp.WaitVisible(`body#reset-password`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}
}

func TestHandleResetPassword_SubmitResetPassword(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	session := &sessions.Session{
		Values: map[interface{}]interface{}{},
	}
	ctx = controller.WithSession(ctx, session)

	harness := envstest.NewServer(t, testDatabaseInstance)

	_, user, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, project.Root()+"/cmd/server/assets", true)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, h)
	handleFunc := c.HandleSubmitResetPassword()

	// success
	func() {
		form := url.Values{}
		form.Set("email", user.Email)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		handleFunc.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close() // likely no-op for test, but we have a presubmit looking for it

		if result.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 300 SeeOther, got %d", result.StatusCode)
		}
	}()
}
