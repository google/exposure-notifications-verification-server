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

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/login"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/gorilla/sessions"
)

func TestHandleSelectRealm_ShowSelectRealm(t *testing.T) {
	t.Parallel()

	t.Run("no_realms", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSelectRealm())

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})
		ctx = controller.WithMemberships(ctx, []*database.Membership{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Body.String(), "not a member of any realms"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})

	t.Run("no_realms_system_admin", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSelectRealm())

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{
			SystemAdmin: true,
		})
		ctx = controller.WithMemberships(ctx, []*database.Membership{})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), "/admin"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})

	t.Run("single_realm", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSelectRealm())

		session := &sessions.Session{
			Values: make(map[interface{}]interface{}),
		}
		realm := &database.Realm{}
		user := &database.User{}

		controller.StoreSessionRealm(session, realm)

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)
		ctx = controller.WithMemberships(ctx, []*database.Membership{
			{
				User:  user,
				Realm: realm,
			},
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Header().Get("Location"), "/login/post-authenticate"; got != want {
			t.Errorf("expected %q to be %q", got, want)
		}
	})

	t.Run("multi_realm", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)

		c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
		handler := harness.WithCommonMiddlewares(c.HandleSelectRealm())

		session := &sessions.Session{
			Values: make(map[interface{}]interface{}),
		}
		realm := &database.Realm{}
		user := &database.User{}

		controller.StoreSessionRealm(session, realm)

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, session)
		ctx = controller.WithUser(ctx, user)
		ctx = controller.WithMemberships(ctx, []*database.Membership{
			{
				User:  user,
				Realm: realm,
			},
			{
				User:  user,
				Realm: &database.Realm{},
			},
			{
				User:  user,
				Realm: &database.Realm{},
			},
		})

		w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusOK; got != want {
			t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
		}
		if got, want := w.Body.String(), "select a realm"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})
}
