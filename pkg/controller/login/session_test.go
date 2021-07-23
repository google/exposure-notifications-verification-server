// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"net/url"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/gorilla/sessions"
)

func TestHandleSession_HandleSubmit(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
	handler := harness.WithCommonMiddlewares(c.HandleCreateSession())

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
	})

	t.Run("missing_id_token", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusBadRequest; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})

	t.Run("missing_session_info", func(t *testing.T) {
		t.Parallel()

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodPost, "/", &url.Values{
			"idToken": []string{"abcd1234"},
		})
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
	})
}
