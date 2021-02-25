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
	"net/http"
	"testing"

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

func TestIssueMalformed(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	realm, err := harness.Database.FindRealm(1)
	if err != nil {
		t.Fatal(err)
	}
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
			name: "issue wrong request type",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeIssue,
			},
			fn:   c.IssueWithUIAuth,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue batch wrong request type",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue API wants authapp",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeIssue,
			},
			fn: c.IssueWithAPIAuth,
			req: api.IssueCodeRequest{
				Phone: "something",
			},
			code: http.StatusInternalServerError, // unauthorized at middleware
		},
		{
			name: "issue batch API wants authapp",
			membership: &database.Membership{
				Realm:       realm,
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithAPIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusInternalServerError, // unauthorized at middleware
		},
		{
			name: "issue no permissions",
			membership: &database.Membership{
				Realm: realm,
				// no permissions
			},
			fn: c.IssueWithUIAuth,
			req: api.IssueCodeRequest{
				Phone: "something",
			},
			code: http.StatusUnauthorized,
		},
		{
			name: "issue batch no permissions",
			membership: &database.Membership{
				Realm: realm,
				// no permissions
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusUnauthorized,
		},
		{
			name: "issue batch, realm not allowed",

			membership: &database.Membership{
				Realm: &database.Realm{
					AllowBulkUpload: false,
				},
				Permissions: rbac.CodeBulkIssue,
			},
			fn:   c.BatchIssueWithUIAuth,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusBadRequest,
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
				t.Errorf("incorrect error code. got %d, want %d", got, want)
			}
		})
	}
}
