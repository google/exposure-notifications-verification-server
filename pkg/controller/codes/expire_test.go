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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/browser"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"

	"github.com/chromedp/chromedp"
)

func TestHandleExpire_ExpireCode(t *testing.T) {
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
		Claimed:       false,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
	}
	if err := harness.Database.SaveVerificationCode(vc, realm); err != nil {
		t.Fatal(err)
	}

	browserCtx := browser.New(t)
	taskCtx, done := context.WithTimeout(browserCtx, project.TestTimeout())
	defer done()

	confirmErrCh := envstest.AutoConfirmDialogs(taskCtx, true)

	if err := chromedp.Run(taskCtx,
		browser.SetCookie(cookie),
		chromedp.Navigate(`http://`+harness.Server.Addr()+`/codes/`+vc.UUID),
		chromedp.WaitVisible(`body#codes-show`, chromedp.ByQuery),

		chromedp.Click(`#code-expire`, chromedp.ByQuery),
		chromedp.WaitVisible(`body#codes-show`, chromedp.ByQuery),
	); err != nil {
		t.Fatal(err)
	}

	if err := <-confirmErrCh; err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	if code, err := realm.FindVerificationCodeByUUID(harness.Database, vc.UUID); err != nil {
		t.Fatal(err)
	} else if code.ExpiresAt.After(now) {
		t.Errorf("expected code expired. got %s but now is %s", code.ExpiresAt, now)
	}
}

func TestHandleExpireAPI_ExpireCode(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, _, _, err := harness.ProvisionAndLogin()
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
		Claimed:       false,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(time.Hour),
		IssuingAppID:  authApp.ID,
	}
	if err := harness.Database.SaveVerificationCode(vc, realm); err != nil {
		t.Fatal(err)
	}

	config := &config.AdminAPIServerConfig{}
	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	c := codes.NewAPI(ctx, config, harness.Database, h)
	handler := c.HandleExpireAPI()

	// not-authorized
	func() {
		b, err := json.Marshal(api.ExpireCodeRequest{UUID: vc.UUID})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close() // likely no-op for test, but we have a presubmit looking for it

		if result.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", result.StatusCode)
		}
	}()

	// successful request
	ctx = controller.WithAuthorizedApp(ctx, authApp)
	func() {
		b, err := json.Marshal(api.ExpireCodeRequest{UUID: vc.UUID})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close() // likely no-op for test, but we have a presubmit looking for it

		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", result.StatusCode)
		}

		now := time.Now().UTC()

		if code, err := realm.FindVerificationCodeByUUID(harness.Database, vc.UUID); err != nil {
			t.Fatal(err)
		} else if code.ExpiresAt.After(now) {
			t.Errorf("expected code expired. got %s but now is %s", code.ExpiresAt, now)
		}
	}()

	// invalid request
	func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", strings.NewReader("invalid request"))
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()
		if result.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 BadRequest, got %d", result.StatusCode)
		}
	}()

	// not-found uuid
	func() {
		b, err := json.Marshal(api.ExpireCodeRequest{UUID: "123e4567-e89b-12d3-a456-426614174000"})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close() // likely no-op for test, but we have a presubmit looking for it

		if result.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 notFound, got %d", result.StatusCode)
		}
	}()
}
