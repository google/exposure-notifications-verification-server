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

package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/admin"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/gorilla/sessions"
)

func TestAdminSMS(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := admin.New(harness.Config, harness.Cacher, harness.Database, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
	handler := middleware.InjectCurrentPath()(c.HandleSMSUpdate())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := admin.New(harness.Config, harness.Cacher, harness.BadDatabase, harness.AuthProvider, harness.RateLimiter, harness.Renderer)
		handler := middleware.InjectCurrentPath()(c.HandleSMSUpdate())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPut, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, w.Header())
		}
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		wantAccountSid := "abc123"
		wantAuthToken := "def456"
		wantFromNumber1 := "+11234567890"
		wantFromNumber2 := "+99999999999"

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"twilio_account_sid":          []string{wantAccountSid},
			"twilio_auth_token":           []string{wantAuthToken},
			"twilio_from_numbers.0.label": []string{"aaa"},
			"twilio_from_numbers.0.value": []string{wantFromNumber1},

			"twilio_from_numbers.1.label": []string{"zzz"},
			"twilio_from_numbers.1.value": []string{wantFromNumber2},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}

		systemSMSConfig, err := harness.Database.SystemSMSConfig()
		if err != nil {
			t.Fatal(err)
		}

		if systemSMSConfig.TwilioAccountSid != wantAccountSid {
			t.Errorf("got: %s, want: %s", systemSMSConfig.TwilioAccountSid, wantAccountSid)
		}
		if systemSMSConfig.TwilioAuthToken != wantAuthToken {
			t.Errorf("got: %s, want: %s", systemSMSConfig.TwilioAuthToken, wantAuthToken)
		}

		smsFromNumbers, err := harness.Database.SMSFromNumbers()
		if err != nil {
			t.Fatal(err)
		}

		if got, want := len(smsFromNumbers), 2; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		aaa := smsFromNumbers[0]
		if got, want := aaa.Label, "aaa"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := aaa.Value, wantFromNumber1; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}

		zzz := smsFromNumbers[1]
		if got, want := zzz.Label, "zzz"; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
		if got, want := zzz.Value, wantFromNumber2; got != want {
			t.Errorf("Expected %q to be %q", got, want)
		}
	})
}
