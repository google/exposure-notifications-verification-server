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
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/jinzhu/gorm"
)

func TestValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tc := testconfig.NewServerConfig(t, TestDatabaseInstance)
	db := tc.Database

	realm := database.NewRealmWithDefaults("Test Realm")
	realm.AllowedTestTypes = database.TestTypeConfirmed
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to save realm: %v", err)
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

	authApp := &database.AuthorizedApp{
		Model: gorm.Model{ID: 123},
	}

	membership := &database.Membership{UserID: 456}

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
			name: "no phone provider",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: symptomDate,
				Phone:       "+somephone",
			},
			httpStatusCode: http.StatusBadRequest,
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
			name: "future date",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: "3020-01-01",
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

			verCode, result := c.populateCode(ctx, &tc.request, authApp, membership, realm)
			if verCode != nil {
				if tc.request.UUID != "" && tc.request.UUID != verCode.UUID {
					t.Errorf("expecting stable client-provided uuid. got %s, want %s", verCode.UUID, tc.request.UUID)
				}
				return
			}
			resp := result.issueCodeResponse()
			if result.httpCode != tc.httpStatusCode {
				t.Errorf("incorrect error code. got %d, want %d", result.httpCode, tc.httpStatusCode)
			}
			if resp.ErrorCode != tc.responseErr {
				t.Errorf("did not receive expected errorCode. got %q, want %q", resp.ErrorCode, tc.responseErr)
			}
		})
	}
}
