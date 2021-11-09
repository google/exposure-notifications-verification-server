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
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestIssue(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
	realm.AllowedTestTypes = database.TestTypeConfirmed | database.TestTypeLikely | database.TestTypeNegative | database.TestTypeUserReport
	realm.AllowBulkUpload = true

	c := issueapi.New(harness.Config, harness.Database, harness.RateLimiter, harness.KeyManager, harness.Renderer)

	cases := []struct {
		name       string
		membership *database.Membership
		fn         func(http.ResponseWriter, *http.Request) *issueapi.IssueResult
		req        interface{}
		code       int
	}{
		{
			name: "issue_wrong_request_type",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeIssue,
			},
			fn:   c.IssueWithUIAuth,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue_batch_wrong_request_type",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue_api_wants_authapp",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeIssue,
			},
			fn: c.IssueWithAPIAuth,
			req: api.IssueCodeRequest{
				Phone: "5005550000",
			},
			code: http.StatusInternalServerError, // unauthorized at middleware
		},
		{
			name: "issue_batch_api_wants_authapp",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithAPIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusInternalServerError, // unauthorized at middleware
		},
		{
			name: "issue_no_permissions",
			membership: &database.Membership{
				Realm: realm,
				// no permissions
			},
			fn: c.IssueWithUIAuth,
			req: api.IssueCodeRequest{
				Phone: "5005550000",
			},
			code: http.StatusUnauthorized,
		},
		{
			name: "issue_batch_no_permissions",
			membership: &database.Membership{
				Realm: realm,
				// no permissions
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusUnauthorized,
		},
		{
			name: "issue_batch_realm_not_allowed",
			membership: &database.Membership{
				Realm: func() *database.Realm {
					realm := database.NewRealmWithDefaults("issue_batch_realm_not_allowed")
					realm.AllowBulkUpload = false
					if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
						t.Fatal(err, realm.ErrorMessages())
					}
					return realm
				}(),
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusBadRequest,
		},
		{
			name: "generated_sms_realm_not_allowed",
			membership: &database.Membership{
				Realm: func() *database.Realm {
					realm := database.NewRealmWithDefaults("generated_sms_realm_not_allowed")
					realm.AllowGeneratedSMS = false
					if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
						t.Fatal(err, realm.ErrorMessages())
					}
					return realm
				}(),
				Permissions: rbac.CodeIssue,
			},
			fn: c.IssueWithUIAuth,
			req: api.IssueCodeRequest{
				Phone:           "5005550000",
				OnlyGenerateSMS: true,
			},
			code: http.StatusBadRequest,
		},
		{
			name: "generated_sms_realm_allowed",
			membership: &database.Membership{
				Realm: func() *database.Realm {
					realm := database.NewRealmWithDefaults("generated_sms_realm_allowed")
					realm.CodeDuration = database.FromDuration(5 * time.Minute)
					realm.CodeLength = 8
					realm.LongCodeDuration = database.FromDuration(15 * time.Minute)
					realm.LongCodeLength = 16
					realm.AllowedTestTypes = database.TestTypeConfirmed
					realm.AllowGeneratedSMS = true
					realm.SMSCountry = "US"
					if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
						t.Fatal(err, realm.ErrorMessages())
					}
					return realm
				}(),
				Permissions: rbac.CodeIssue,
			},
			fn: c.IssueWithUIAuth,
			req: api.IssueCodeRequest{
				Phone:           "5005550000",
				SymptomDate:     time.Now().UTC().Format(project.RFC3339Date),
				TestType:        "confirmed",
				OnlyGenerateSMS: true,
			},
			code: http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			ctx = controller.WithMembership(ctx, tc.membership)

			w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", tc.req)
			tc.fn(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
			}
		})
	}
}
