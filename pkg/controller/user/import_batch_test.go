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

package user_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

func TestHandleImportBatch(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, testUser, session, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}
	ctx = controller.WithSession(ctx, session)

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}
	c := user.New(harness.AuthProvider, harness.Cacher, harness.Database, h)
	handler := c.HandleImportBatch()

	envstest.ExerciseMembershipMissing(t, handler)
	envstest.ExercisePermissionMissing(t, handler)

	ctx = controller.WithMembership(ctx, &database.Membership{
		Realm:       realm,
		User:        testUser,
		Permissions: rbac.UserWrite,
	})

	// success
	func() {
		b, err := json.Marshal(api.UserBatchRequest{
			Users: []api.BatchUser{
				{
					Email: "test@example.com",
					Name:  "batch tester",
				},
			},
			SendInvites: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", result.StatusCode)
		}
	}()

	// invalid user
	func() {
		b, err := json.Marshal(api.UserBatchRequest{
			Users: []api.BatchUser{
				{
					Email: "thisisfine@example.com",
					Name:  "valid tester",
				},
				{
					Email: "", // required user field
					Name:  "invalid tester",
				},
			},
			SendInvites: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", result.StatusCode)
		}
	}()
}
