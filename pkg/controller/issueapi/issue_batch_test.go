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

package issueapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/google/exposure-notifications-verification-server/pkg/testsuite"
)

func TestIssueBatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSuite := testsuite.NewIntegrationSuite(t, ctx)
	adminClient, err := testSuite.NewAdminAPIClient(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	db := testSuite.DB
	realm := testSuite.Realm

	realm.AllowedTestTypes = database.TestTypeConfirmed
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
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

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)
	tzMinOffset := 0

	cases := []struct {
		name           string
		request        api.BatchIssueCodeRequest
		response       api.BatchIssueCodeResponse
		httpStatusCode int
	}{
		{
			name: "success",
			request: api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{
						TestType:    "confirmed",
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
				},
			},
			response: api.BatchIssueCodeResponse{
				Codes: []*api.IssueCodeResponse{
					{
						// success
					},
				},
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name: "all failure",
			request: api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{
						TestType:    "negative", // this realm only supports confirmed
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
				},
			},
			response: api.BatchIssueCodeResponse{
				Codes: []*api.IssueCodeResponse{
					{
						ErrorCode: api.ErrUnsupportedTestType,
					},
				},
				ErrorCode: api.ErrUnsupportedTestType,
			},
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "partial success",
			request: api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{
						TestType:    "confirmed",
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
					{
						TestType:    "invalid - fail",
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
					{
						TestType:    "confirmed",
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
						UUID:        existingCode.UUID,
					},
				},
			},
			response: api.BatchIssueCodeResponse{
				Codes: []*api.IssueCodeResponse{
					{
						// success
					},
					{
						ErrorCode: api.ErrInvalidTestType,
					},
					{
						ErrorCode: api.ErrUUIDAlreadyExists,
					},
				},
				ErrorCode: api.ErrInvalidTestType,
			},
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "batch size limit",
			request: api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{TestType: "1"},
					{TestType: "2"},
					{TestType: "3"},
					{TestType: "4"},
					{TestType: "5"},
					{TestType: "6"},
					{TestType: "7"},
					{TestType: "8"},
					{TestType: "9"},
					{TestType: "10"},
					{TestType: "11"},
				},
			},
			response:       api.BatchIssueCodeResponse{},
			httpStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			statusCode, resp, err := adminClient.BatchIssueCode(tc.request)
			if err != nil {
				t.Fatal(err)
			}

			// Check outer error
			if statusCode != tc.httpStatusCode {
				t.Errorf("incorrect error code. got %d, want %d", statusCode, tc.httpStatusCode)
			}
			if resp.ErrorCode != tc.response.ErrorCode {
				t.Errorf("did not receive expected outer errorCode. got %s, want %v", resp.ErrorCode, tc.response.ErrorCode)
			}

			for i, issuedCode := range resp.Codes {
				if issuedCode.ErrorCode != tc.response.Codes[i].ErrorCode {
					t.Errorf("did not receive expected inner errorCode. got %s, want %v", issuedCode.ErrorCode, tc.response.Codes[i].ErrorCode)
				}

				if tc.request.Codes[i].UUID != "" && tc.response.Codes[i].UUID != issuedCode.UUID {
					t.Errorf("expected stable client-issued UUID. got %s, want %v", issuedCode.UUID, tc.response.Codes[i].UUID)
				}
			}
		})
	}
}
