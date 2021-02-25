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

package login_test

import (
	"net/http"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/gorilla/sessions"
)

func TestHandleVerifyEmailSend_ShowVerifyEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
	handler := c.HandleShowVerifyEmail()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{
			Values: make(map[interface{}]interface{}),
		})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})
}

func TestHandleVerifyEmailSend_SubmitVerifyEmail(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
	handler := c.HandleSubmitVerifyEmail()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		realm, err := harness.Database.FindRealm(1)
		if err != nil {
			t.Fatal(err)
		}

		user, err := harness.Database.FindUser(1)
		if err != nil {
			t.Fatal(err)
		}

		emailConfig := &database.EmailConfig{
			RealmID:      realm.ID,
			ProviderType: email.ProviderTypeNoop,
			SMTPAccount:  "noreply@sendemails.meh",
			SMTPPassword: "my-secret-ref",
			SMTPHost:     "smtp.sendemails.meh",
		}
		if err := harness.Database.SaveEmailConfig(emailConfig); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, user)
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm: realm,
			User:  user,
		})

		w, r := envstest.BuildJSONRequest(ctx, t, http.MethodPost, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})
}
