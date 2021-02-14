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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestIssue(t *testing.T) {
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

	symptomDate := time.Now().UTC().Add(-48 * time.Hour).Format(project.RFC3339Date)

	h, err := render.New(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}

	c := issueapi.New(harness.Config, harness.Database, harness.RateLimiter, harness.KeyManager, h)
	handler := c.HandleIssueAPI()

	cases := []struct {
		name           string
		request        *api.IssueCodeRequest
		responseErr    string
		httpStatusCode int

		err bool
	}{
		{
			name: "success",
			request: &api.IssueCodeRequest{
				TestType:    "confirmed",
				SymptomDate: symptomDate,
			},
			httpStatusCode: http.StatusOK,
		},
		// {
		// 	name: "failure",
		// 	request: &api.IssueCodeRequest{
		// 		TestType:    "confirmed",
		// 		SymptomDate: "invalid date",
		// 	},
		// 	responseErr:    api.ErrUnparsableRequest,
		// 	httpStatusCode: http.StatusBadRequest,
		// },
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			if err := json.NewEncoder(&b).Encode(tc.request); err != nil {
				t.Fatal(err)
			}

			ctx := controller.WithRealm(ctx, realm)
			ctx = controller.WithAuthorizedApp(ctx, authApp)
			r, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", &b)
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Accept", "application/json")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

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
