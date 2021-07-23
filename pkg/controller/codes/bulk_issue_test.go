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

package codes_test

import (
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/codes"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"github.com/gorilla/sessions"
)

func TestRenderBulkIssue(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := codes.NewServer(harness.Config, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleBulkIssue())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		c := codes.NewServer(harness.Config, harness.BadDatabase, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleBulkIssue())

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AllowBulkUpload: true,
			},
			User:        &database.User{},
			Permissions: rbac.LegacyRealmAdmin | rbac.CodeBulkIssue,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
	})

	t.Run("not_enabled", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AllowBulkUpload: false,
			},
			User:        &database.User{},
			Permissions: rbac.LegacyRealmAdmin | rbac.CodeBulkIssue,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AllowBulkUpload: true,
			},
			User:        &database.User{},
			Permissions: rbac.LegacyRealmAdmin | rbac.CodeBulkIssue,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("with_sms", func(t *testing.T) {
		t.Parallel()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		realm.AllowBulkUpload = true
		if err := harness.Database.SaveRealm(realm, database.SystemTest); err != nil {
			t.Fatal(err)
		}

		sms := &database.SMSConfig{
			RealmID:          realm.ID,
			ProviderType:     sms.ProviderType("TWILIO"),
			TwilioAccountSid: "abc123",
			TwilioAuthToken:  "def123",
			TwilioFromNumber: "+11234567890",
		}
		if err := harness.Database.SaveSMSConfig(sms); err != nil {
			t.Fatalf("failed to save SMSConfig: %v", err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: &database.Realm{
				AllowBulkUpload: true,
			},
			User:        &database.User{},
			Permissions: rbac.LegacyRealmAdmin | rbac.CodeBulkIssue,
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d: %s", got, want, w.Body.String())
		}
	})
}
