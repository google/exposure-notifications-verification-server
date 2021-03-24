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

package issueapi_test

import (
	"crypto/rand"
	"encoding/base64"
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

func TestUserReport(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AllowBulkUpload = true
	realm.AllowedTestTypes = database.TestTypeConfirmed
	realm.AddUserReportToAllowedTestTypes()
	if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	smsConfig := &database.SMSConfig{
		RealmID:      realm.ID,
		ProviderType: sms.ProviderTypeNoop,
	}
	if err := harness.Database.SaveSMSConfig(smsConfig); err != nil {
		t.Fatal(err)
	}

	authApp := &database.AuthorizedApp{
		Name:       "Appy",
		APIKeyType: database.APIKeyTypeAdmin,
	}
	if _, err := realm.CreateAuthorizedApp(harness.Database, authApp, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)

	nonceBytes := make([]byte, database.NonceLength)
	_, err = rand.Read(nonceBytes)
	if err != nil {
		t.Fatal(err)
	}
	nonce := base64.StdEncoding.EncodeToString(nonceBytes)

	c := issueapi.New(harness.Config, harness.Database, harness.RateLimiter, harness.KeyManager, harness.Renderer)
	handler := c.HandleUserReport()

	cases := []struct {
		name           string
		request        *api.UserReportRequest
		responseErr    string
		httpStatusCode int

		err bool
	}{
		{
			name: "success",
			request: &api.UserReportRequest{
				SymptomDate: symptomDate,
				Phone:       "+12068675309",
				Nonce:       nonce,
			},
			httpStatusCode: http.StatusOK,
		},
		{
			name: "too_soon",
			request: &api.UserReportRequest{
				SymptomDate: symptomDate,
				Phone:       "+12068675309",
				Nonce:       nonce,
			},
			httpStatusCode: http.StatusConflict,
			responseErr:    "user_report_try_later",
		},
		{
			name: "missing_phone",
			request: &api.UserReportRequest{
				SymptomDate: symptomDate,
				Nonce:       nonce,
			},
			httpStatusCode: http.StatusBadRequest,
			responseErr:    "missing_phone",
		},
		{
			name: "missing_nonce",
			request: &api.UserReportRequest{
				SymptomDate: symptomDate,
			},
			httpStatusCode: http.StatusBadRequest,
			responseErr:    "missing_nonce",
		},
		{
			name: "failure",
			request: &api.UserReportRequest{
				SymptomDate: "invalid date",
				Nonce:       nonce,
				Phone:       "+12068675309",
			},
			responseErr:    api.ErrUnparsableRequest,
			httpStatusCode: http.StatusBadRequest,
		},
		{
			name: "failure",
			request: &api.UserReportRequest{
				SymptomDate: "invalid date",
				Nonce:       "..45",
				Phone:       "+12068675309",
			},
			responseErr:    api.ErrUnparsableRequest,
			httpStatusCode: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			// not parallel so that the tests interfer with each other.

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

			if got, want := apiResp.ErrorCode, tc.responseErr; got != want {
				t.Errorf("expected %#v to be %#v: %#v", got, want, apiResp)
			}
		})
	}
}
