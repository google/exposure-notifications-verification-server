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
	"encoding/json"
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

func TestIssueBatch(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AllowBulkUpload = true
	realm.AllowedTestTypes = database.TestTypeConfirmed
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		Name:       "Appy",
		APIKeyType: database.APIKeyTypeAdmin,
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderTypeNoop,
	}
	if err := harness.Database.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)
	tzMinOffset := 0

	c := issueapi.New(harness.Config, harness.Database, harness.RateLimiter, harness.KeyManager, harness.Renderer)
	handler := c.HandleBatchIssueAPI()

	cases := []struct {
		name           string
		request        *api.BatchIssueCodeRequest
		response       *api.BatchIssueCodeResponse
		httpStatusCode int
	}{
		{
			name: "success",
			request: &api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{
						TestType:    "confirmed",
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
				},
			},
			response: &api.BatchIssueCodeResponse{
				Codes: []*api.IssueCodeResponse{
					{
						// success
					},
				},
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name: "all_failure",
			request: &api.BatchIssueCodeRequest{
				Codes: []*api.IssueCodeRequest{
					{
						TestType:    "negative", // this realm only supports confirmed
						SymptomDate: symptomDate,
						TZOffset:    float32(tzMinOffset),
					},
				},
			},
			response: &api.BatchIssueCodeResponse{
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
			name: "partial_success",
			request: &api.BatchIssueCodeRequest{
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
						SymptomDate: "unparsable date",
						TZOffset:    float32(tzMinOffset),
					},
				},
			},
			response: &api.BatchIssueCodeResponse{
				Codes: []*api.IssueCodeResponse{
					{
						// success
					},
					{
						ErrorCode: api.ErrInvalidTestType,
					},
					{
						ErrorCode: api.ErrUnparsableRequest,
					},
				},
				ErrorCode: api.ErrInvalidTestType,
			},
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "batch_size_limit",
			request: &api.BatchIssueCodeRequest{
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
			response:       &api.BatchIssueCodeResponse{},
			httpStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			ctx = controller.WithRealm(ctx, realm)
			ctx = controller.WithAuthorizedApp(ctx, authApp)

			w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", tc.request)
			handler.ServeHTTP(w, r)

			if got, want := w.Code, tc.httpStatusCode; got != want {
				t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
			}

			var apiResp api.BatchIssueCodeResponse
			if err := json.NewDecoder(w.Body).Decode(&apiResp); err != nil {
				t.Fatal(err)
			}

			if got, want := apiResp.ErrorCode, tc.response.ErrorCode; got != want {
				t.Errorf("expected %#v to be %#v: %#v", got, want, apiResp)
			}

			for i, issuedCode := range apiResp.Codes {
				if got, want := issuedCode.ErrorCode, tc.response.Codes[i].ErrorCode; got != want {
					t.Errorf("bad inner error code, expected %#v to be %#v", got, want)
				}
			}
		})
	}
}
