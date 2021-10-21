// Copyright 2021 the Exposure Notifications Verification Server authors
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

package webhooks_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/webhooks"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestHandleTwilio(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}

	realmWithSMS := database.NewRealmWithDefaults("realm-with-sms")
	if err := harness.Database.SaveRealm(realmWithSMS, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:          realmWithSMS.ID,
		ProviderType:     sms.ProviderTypeTwilio,
		TwilioAccountSid: "abc123",
		TwilioFromNumber: "+15005550006",
		TwilioAuthToken:  "abc123",
	}
	if err := harness.Database.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	c := webhooks.New(harness.Cacher, harness.Database, harness.Renderer)

	cases := []struct {
		name     string
		headers  *http.Header
		body     string
		realmID  uint
		wantLog  string
		wantCode int
	}{
		{
			name:     "missing_signature",
			headers:  nil,
			wantLog:  "request is missing signature header",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad_form",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "this is&;my=invalid= r;equest body=",
			wantLog:  "failed to parse form",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "wrong_level",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "valid=true",
			wantLog:  "request Level is not ERROR",
			wantCode: http.StatusOK,
		},
		{
			name:     "wrong_payload_type",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=ERROR&PayloadType=application/xml",
			wantLog:  "request PayloadType is not application/json",
			wantCode: http.StatusOK,
		},
		{
			name:     "too_big",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload=" + strings.Repeat("this_is_my_message", 1000),
			wantLog:  "failed to decode Payload",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "payload_missing_error_code",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload={\"hello\":\"there\"}",
			wantLog:  "got payload, but error_code is empty",
			wantCode: http.StatusOK,
		},
		{
			name:     "bad_realm_id",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload={\"error_code\":\"E00007\"}",
			realmID:  12345,
			wantLog:  "failed to lookup realm",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "no_sms_config",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload={\"error_code\":\"E00007\"}",
			realmID:  realm.ID,
			wantLog:  "failed to lookup realm sms config",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad_account_sid",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload={\"error_code\":\"E00007\"}&AccountSid=def456",
			realmID:  realmWithSMS.ID,
			wantLog:  "twilio account sid mismatch",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "bad_signature",
			headers:  &http.Header{"X-Twilio-Signature": []string{"abc123"}},
			body:     "Level=error&PayloadType=application/json&Payload={\"error_code\":\"E00007\"}&AccountSid=abc123",
			realmID:  realmWithSMS.ID,
			wantLog:  "signature mismatch",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "success",
			headers:  &http.Header{"X-Twilio-Signature": []string{"Ht5oZoK1sm7KmEUfMFYfslEY+KE="}},
			body:     "Level=error&PayloadType=application/json&Payload={\"error_code\":\"E00007\"}&AccountSid=abc123",
			realmID:  realmWithSMS.ID,
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create custom logger which we can observe for log messages
			logCore, logObserver := observer.New(zap.DebugLevel)
			ctx := logging.WithLogger(ctx, zap.New(logCore).Sugar())

			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			r = r.Clone(ctx)

			if id := tc.realmID; id != 0 {
				r = mux.SetURLVars(r, map[string]string{"realm_id": fmt.Sprintf("%d", id)})
			}

			// Set custom headers, if present
			if tc.headers != nil {
				r.Header = *tc.headers
			}
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()

			c.HandleTwilio().ServeHTTP(w, r)

			if want := tc.wantLog; want != "" {
				messages := logObserver.All()
				if len(messages) == 0 {
					t.Fatal("expected log messages, got none")
				}

				if got := messages[0].Message; got != want {
					t.Errorf("expected %q to be %q", got, want)
				}
			}

			if got, want := w.Code, tc.wantCode; got != want {
				t.Errorf("expected response to be %d, got %d: %s", want, got, w.Body.String())
			}
		})
	}
}

func TestComputeSignature(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	// The data in this test comes from a real webhook invocation. Yes, the auth
	// token below is real. Yes, it has been rotated and is no longer valid.
	expSignature := "oT6J/5wQ3mmQfOQi5cm1FoxbA70="
	authToken := "1d2a" + "7f480d4e6913a72f2febbe63379d"

	vals := make(url.Values)
	vals.Add("ParentAccountSid", "ACffcbcc79af64899e720a5076b4e6b217")
	vals.Add("Payload", `{"resource_sid":"SMba2e5f029d1b1630bbf01232ff9b814c","service_sid":"SM736f35d231bd230a80c8929e50a2c24c","error_code":"30007"}`)
	vals.Add("PayloadType", "application/json")
	vals.Add("AccountSid", "AC9a5a39b6ac47a0061bb00da10efd8264")
	vals.Add("Timestamp", "2021-10-12T23:43:53.289Z")
	vals.Add("Level", "ERROR")
	vals.Add("Sid", "NO1436992f15326b2603d79db507cedba2")

	body := strings.NewReader(vals.Encode())
	req := httptest.NewRequest(http.MethodPost, "https://example.com", body)
	req = req.Clone(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	x, err := webhooks.ComputeSignature(req, authToken)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := x, expSignature; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}
