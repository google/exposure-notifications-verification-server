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

package issueapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest/testconfig"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func TestIssueCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tc := testconfig.NewServerConfig(t, TestDatabaseInstance)
	db := tc.Database

	realm := database.NewRealmWithDefaults("Test Realm")
	realm.AllowBulkUpload = true
	realm.AllowedTestTypes = database.TestTypeConfirmed
	ctx = controller.WithRealm(ctx, realm)
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to save realm: %v", err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderType(sms.ProviderTypeNoop),
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
		LongExpiresAt: time.Now().Add(time.Hour),
	}
	if err := db.SaveVerificationCode(existingCode, time.Hour); err != nil {
		t.Fatal(err)
	}

	membership := &database.Membership{
		RealmID:     realm.ID,
		Realm:       realm,
		Permissions: rbac.CodeBulkIssue,
	}

	ctx = controller.WithMembership(ctx, membership)
	c := New(tc.Config, db, tc.RateLimiter, nil)

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)

	cases := []struct {
		name           string
		request        api.IssueCodeRequest
		responseErr    string
		httpStatusCode int
	}{
		{
			name: "success",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: symptomDate,
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name: "unsupported test type",
			request: api.IssueCodeRequest{
				TestType:    "negative", // this realm only supports confirmed
				SymptomDate: symptomDate,
			},
			responseErr:    api.ErrUnsupportedTestType,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "invalid test type",
			request: api.IssueCodeRequest{
				TestType:    "invalid",
				SymptomDate: symptomDate,
			},
			responseErr:    api.ErrInvalidTestType,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "no test date",
			request: api.IssueCodeRequest{
				TestType: "confirmed",
			},
			responseErr:    api.ErrMissingDate,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "unparsable test date",
			request: api.IssueCodeRequest{
				TestType: "confirmed",
				TestDate: "invalid date",
			},
			responseErr:    api.ErrUnparsableRequest,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "really old test date",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: "1988-09-14",
			},
			responseErr:    api.ErrInvalidDate,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "conflict",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: symptomDate,
				UUID:        existingCode.UUID,
			},
			responseErr:    api.ErrUUIDAlreadyExists,
			httpStatusCode: http.StatusConflict,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := c.issueOne(ctx, &tc.request, nil, membership, realm)
			resp := result.issueCodeResponse()

			if result.HTTPCode != tc.httpStatusCode {
				t.Errorf("incorrect error code. got %d, want %d", result.HTTPCode, tc.httpStatusCode)
			}
			if resp.ErrorCode != tc.responseErr {
				t.Errorf("did not receive expected errorCode. got %q, want %q", resp.ErrorCode, tc.responseErr)
			}
		})
	}
}
