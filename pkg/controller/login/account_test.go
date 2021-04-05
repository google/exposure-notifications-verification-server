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
	"strings"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/gorilla/sessions"
)

func TestHandleAccount_ShowAccount(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleAccountSettings())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		session := &sessions.Session{}
		if err := harness.AuthProvider.StoreSession(ctx, session, &auth.SessionInfo{
			Data: map[string]interface{}{
				"email":          "you@example.com",
				"email_verified": true,
				"mfa_enabled":    false,
				"revoked":        false,
			},
		}); err != nil {
			t.Fatal(err)
		}

		ctx := ctx
		ctx = controller.WithSession(ctx, session)

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Email address is verified"; !strings.Contains(got, want) {
			t.Errorf("Expected %s to contain %s", got, want)
		}
		if got, want := w.Body.String(), "(MFA) is disabled"; !strings.Contains(got, want) {
			t.Errorf("Expected %s to contain %s", got, want)
		}
	})
}
