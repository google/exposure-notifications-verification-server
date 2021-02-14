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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
)

func TestHandleResetPassword(t *testing.T) {
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
	router := mux.NewRouter()
	router.Handle("/{id:[0-9]+}/reset-password", c.HandleResetPassword()).Methods(http.MethodPost)

	// Needs membership
	func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("/%d/reset-password", testUser.ID), strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400 BadRequest, got %d", result.StatusCode)
		}
	}()

	// Needs userwrite
	func() {
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: realm,
			User:  testUser,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("/%d/reset-password", testUser.ID), strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", result.StatusCode)
		}
	}()

	// success
	func() {
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        testUser,
			Permissions: rbac.UserWrite,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("/%d/reset-password", testUser.ID), strings.NewReader(""))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		result := w.Result()
		defer result.Body.Close()

		if result.StatusCode != http.StatusSeeOther {
			t.Errorf("expected status 303 SeeOther, got %d", result.StatusCode)
		}
	}()
}
