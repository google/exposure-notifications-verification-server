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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestIssueMaintenanceMode(t *testing.T) {
	t.Parallel()
	tc := envstest.NewServerConfig(t, testDatabaseInstance)
	ctx := context.Background()
	r, err := render.New(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}
	c := issueapi.New(tc.Config, tc.Database, tc.RateLimiter, r)
	tc.Config.MaintenanceMode = true

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) *issueapi.IssueResult
	}{
		{
			name: "issue",
			fn:   c.HandleIssueFn,
		},
		{
			name: "issue batch",
			fn:   c.HandleBatchIssueFn,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			tc.fn(recorder, nil)

			result := recorder.Result()
			if result.StatusCode != http.StatusTooManyRequests {
				t.Errorf("incorrect error code. got %d, want %d", result.StatusCode, http.StatusTooManyRequests)
			}
		})
	}
}

func TestIssueMalformed(t *testing.T) {
	t.Parallel()
	tc := envstest.NewServerConfig(t, testDatabaseInstance)
	ctx := context.Background()
	r, err := render.New(ctx, "", true)
	if err != nil {
		t.Fatal(err)
	}
	c := issueapi.New(tc.Config, tc.Database, tc.RateLimiter, r)

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) *issueapi.IssueResult
		req  interface{}
		code int
	}{
		{
			name: "issue",
			fn:   c.HandleIssueFn,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue batch",
			fn:   c.HandleBatchIssueFn,
			req:  api.VerifyCodeRequest{}, // not issue
			code: http.StatusBadRequest,
		},
		{
			name: "issue",
			fn:   c.HandleIssueFn,
			req: api.IssueCodeRequest{
				Phone: "something",
			},
			code: http.StatusUnauthorized,
		},
		{
			name: "issue batch",
			fn:   c.HandleBatchIssueFn,
			req:  api.BatchIssueCodeRequest{},
			code: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reader io.Reader
			if tc.req != nil {
				b, err := json.Marshal(tc.req)
				if err != nil {
					t.Fatal(err)
				}
				reader = bytes.NewBuffer(b)
			}
			req, err := http.NewRequest("GET", "http://example.com", reader)
			req.Header.Add("content-type", "application/json")
			if err != nil {
				t.Fatal(err)
			}

			recorder := httptest.NewRecorder()
			tc.fn(recorder, req)

			result := recorder.Result()
			if result.StatusCode != tc.code {
				t.Errorf("incorrect error code. got %d, want %d", result.StatusCode, tc.code)
			}
		})
	}
}
