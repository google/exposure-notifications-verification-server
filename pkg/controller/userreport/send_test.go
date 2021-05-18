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

package userreport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func TestBuildAndSignPayloadForWebhook(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		secret  string
		body    interface{}
		expBody string
		expMac  string
	}{
		{
			name:    "empty",
			secret:  "",
			body:    struct{}{},
			expBody: "{}",
			expMac:  "bc7b0c6253e31736a26b597695004434377f48ccf1c5b97a44870c8c929495465b6693b4a7097a8ac6b8ee2f744f4ba6f6b52fcdb74cd5a4ec5611a89024b1f9",
		},
		{
			name:    "with_secret",
			secret:  "foobarbaz",
			body:    struct{}{},
			expBody: "{}",
			expMac:  "e0baf34f99c5eadd808ff0d34326a634a8d8277f1f8839876d6fe7908c6849d559ca3593e16ca68106f93a7f791fec7bcf2d702a6036d84959df32de33f6a1b6",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, mac, err := buildAndSignPayloadForWebhook(tc.secret, tc.body)
			if err != nil {
				t.Fatal(err)
			}

			if got, want := string(b), tc.expBody; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}

			if got, want := mac, tc.expMac; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}

func TestSendWebhookRequest(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if v := r.Header.Get("Content-Type"); v != "application/json" {
			t.Errorf("expected content-type to be application/json: %q", w.Header())
		}

		if r.Header.Get("X-Signature") == "" {
			t.Errorf("expected signature to be present")
		}

		var m map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Errorf("bad json: %s", err)
		}

		if got, want := m["generatedSMS"], "TODO"; got != want {
			t.Errorf("expected %q to be %q: %v", got, want, m)
		}
		if got, want := m["phone"], "+15005550000"; got != want {
			t.Errorf("expected %q to be %q: %v", got, want, m)
		}
		if got, want := m["code"], "abcd1234"; got != want {
			t.Errorf("expected %q to be %q: %v", got, want, m)
		}
		if _, ok := m["uuid"]; !ok {
			t.Errorf("expected uuid to be present: %v", m)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	realm := &database.Realm{
		UserReportWebhookURL:    srv.URL,
		UserReportWebhookSecret: "super-secret",
	}

	if err := sendWebhookRequest(ctx, client, realm, &issueapi.IssueResult{
		VerCode: &database.VerificationCode{
			RealmID:     realm.ID,
			PhoneNumber: "+15005550000",
			Code:        "abcd1234",
		},
		GeneratedSMS: "TODO",
	}); err != nil {
		t.Fatal(err)
	}
}
