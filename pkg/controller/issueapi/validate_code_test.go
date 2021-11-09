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

	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/jinzhu/gorm"
)

type buildCodeValidation func(t testing.TB, vCode *database.VerificationCode)

func TestValidate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)
	db := harness.Database

	realm, err := db.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AllowedTestTypes = database.TestTypeConfirmed
	realm.AllowGeneratedSMS = true
	realm.CodeDuration = database.FromDuration(15 * time.Minute)
	realm.LongCodeDuration = database.FromDuration(24 * time.Hour)
	if err := db.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatalf("failed to update realm: %v", err)
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
	if err := realm.SaveVerificationCode(db, existingCode); err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		Model: gorm.Model{ID: 123},
	}

	c := issueapi.New(harness.Config, db, harness.RateLimiter, harness.KeyManager, nil)

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)

	maxDate := timeutils.UTCMidnight(time.Now())
	minDate := timeutils.Midnight(maxDate.Add(-1 * harness.Config.IssueConfig().AllowedSymptomAge))

	cases := []struct {
		name           string
		request        api.IssueCodeRequest
		responseErr    string
		httpStatusCode int
		vcValidation   buildCodeValidation
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
			name: "no_phone_provider",
			request: api.IssueCodeRequest{
				TestType:        "confirmed",
				SymptomDate:     symptomDate,
				Phone:           "5005550000",
				OnlyGenerateSMS: false,
			},
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "unsupported_test_type",
			request: api.IssueCodeRequest{
				TestType:    "negative", // this realm only supports confirmed
				SymptomDate: symptomDate,
			},
			responseErr:    api.ErrUnsupportedTestType,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "invalid_test_type",
			request: api.IssueCodeRequest{
				TestType:    "invalid",
				SymptomDate: symptomDate,
			},
			responseErr:    api.ErrInvalidTestType,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "no_test_date",
			request: api.IssueCodeRequest{
				TestType: "confirmed",
			},
			responseErr:    api.ErrMissingDate,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "unparsable_test_date",
			request: api.IssueCodeRequest{
				TestType: "confirmed",
				TestDate: "invalid date",
			},
			responseErr:    api.ErrUnparsableRequest,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "really_old_test_date",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: "1988-09-14",
			},
			responseErr:    api.ErrInvalidDate,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "future_date",
			request: api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: "3020-01-01",
			},
			responseErr:    api.ErrInvalidDate,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "test_older_than_minDate",
			request: api.IssueCodeRequest{
				TestType: "confirmed",
				TestDate: minDate.Add(-12 * time.Hour).Format(project.RFC3339Date),
				TZOffset: -5, // we loosen an extra day for this
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
		{
			name: "only_generate_sms",
			request: api.IssueCodeRequest{
				TestType:        "confirmed",
				TestDate:        symptomDate,
				OnlyGenerateSMS: true,
				Phone:           "+12068675309",
			},
			httpStatusCode: http.StatusOK,
			vcValidation: func(t testing.TB, vCode *database.VerificationCode) {
				t.Helper()

				if !vCode.LongExpiresAt.After(vCode.ExpiresAt.Add(time.Second)) {
					t.Errorf("long_expires_at (%v) is not after expires_at (%v)", vCode.LongExpiresAt, vCode.ExpiresAt)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			ctx = controller.WithAuthorizedApp(ctx, authApp)
			ctx = controller.WithMembership(ctx, &database.Membership{UserID: 456})

			verCode, result := c.BuildVerificationCode(ctx, &issueapi.IssueRequestInternal{IssueRequest: &tc.request}, realm)
			if tc.vcValidation != nil {
				tc.vcValidation(t, verCode)
			}
			if verCode != nil {
				if tc.request.UUID != "" && tc.request.UUID != verCode.UUID {
					t.Errorf("expecting stable client-provided uuid. got %s, want %s", verCode.UUID, tc.request.UUID)
				}
				if tc.request.TestDate != "" && verCode.TestDate == nil {
					t.Errorf("No test date. got %s, want %s", verCode.TestDate, tc.request.TestDate)
				}
				if tc.request.SymptomDate != "" && verCode.SymptomDate == nil {
					t.Errorf("No symptom date. got %s, want %s", verCode.TestDate, tc.request.TestDate)
				}
				return
			}
			resp := result.IssueCodeResponse()
			if result.HTTPCode != tc.httpStatusCode {
				t.Errorf("incorrect error code. got %d, want %d", result.HTTPCode, tc.httpStatusCode)
			}
			if resp.ErrorCode != tc.responseErr {
				t.Errorf("did not receive expected errorCode. got %q, want %q", resp.ErrorCode, tc.responseErr)
			}
		})
	}
}
