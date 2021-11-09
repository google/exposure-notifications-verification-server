// Copyright 2020 the Exposure Notifications Verification Server authors
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

package issueapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestIssueOne(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)
	db := harness.Database

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithRealm(ctx, realm)

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderTypeNoop,
	}
	if err := db.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	existingCode := &database.VerificationCode{
		RealmID:       realm.ID,
		Code:          "00000001",
		LongCode:      "00000001ABC",
		Claimed:       true,
		TestType:      "confirmed",
		ExpiresAt:     time.Now().Add(time.Hour),
		LongExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := realm.SaveVerificationCode(db, existingCode); err != nil {
		t.Fatal(err)
	}

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)

	c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, harness.Renderer)

	cases := []struct {
		name           string
		request        *issueapi.IssueRequestInternal
		responseErr    string
		httpStatusCode int
	}{
		{
			name: "confirmed_test",
			request: &issueapi.IssueRequestInternal{
				IssueRequest: &api.IssueCodeRequest{
					TestType:    "confirmed",
					SymptomDate: symptomDate,
					Phone:       "+15005550006",
				},
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name: "invalid_test_type",
			request: &issueapi.IssueRequestInternal{
				IssueRequest: &api.IssueCodeRequest{
					TestType:    "invalid",
					SymptomDate: symptomDate,
				},
			},
			responseErr:    api.ErrInvalidTestType,
			httpStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := c.IssueOne(ctx, tc.request)
			resp := result.IssueCodeResponse()

			if result.HTTPCode != tc.httpStatusCode {
				t.Errorf("incorrect error code. got %d, want %d", result.HTTPCode, tc.httpStatusCode)
			}
			if resp.ErrorCode != tc.responseErr {
				t.Errorf("did not receive expected errorCode. got %q, want %q", resp.ErrorCode, tc.responseErr)
			}

			if tc.responseErr == "" && tc.request.IssueRequest.Phone != "" && resp.ExpiresAt == resp.LongExpiresAt {
				t.Errorf("Long expiry should be longer than short when a phone is provided.")
			}
		})
	}
}
