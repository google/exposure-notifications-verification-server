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

package realmadmin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/realmadmin"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandleSettings_SMS(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	c := realmadmin.New(harness.Config, harness.Database, harness.RateLimiter, harness.Renderer)
	handler := middleware.InjectCurrentPath()(c.HandleSettings())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("updates", func(t *testing.T) {
		t.Parallel()

		wantAccountSid := "abc123"
		wantAuthToken := "def456"
		wantFromNumber := "+11234567890"

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        &database.User{},
			Permissions: rbac.SettingsRead | rbac.SettingsWrite,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"sms":                []string{"1"},
			"twilio_account_sid": []string{wantAccountSid},
			"twilio_auth_token":  []string{wantAuthToken},
			"twilio_from_number": []string{wantFromNumber},

			"sms_text_label_0":    []string{"Default SMS template"},
			"sms_text_template_0": []string{"This is your [code]"},

			"sms_text_label_1":    []string{"Custom SMS template"},
			"sms_text_template_1": []string{"This is your [longcode]"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}

		smsConfig, err := realm.SMSConfig(harness.Database)
		if err != nil {
			t.Fatal(err)
		}
		if smsConfig.TwilioAccountSid != wantAccountSid {
			t.Errorf("expected %q to be %q", smsConfig.TwilioAccountSid, wantAccountSid)
		}
		if smsConfig.TwilioAuthToken != wantAuthToken {
			t.Errorf("expected %q to be %q", smsConfig.TwilioAuthToken, wantAuthToken)
		}
		if smsConfig.TwilioFromNumber != wantFromNumber {
			t.Errorf("expected %q to be %q", smsConfig.TwilioFromNumber, wantFromNumber)
		}
	})
}
