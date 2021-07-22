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

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/middleware"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
)

func TestLoadCurrentMembership(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	loadCurrentMembership := middleware.LoadCurrentMembership(h)

	user := &database.User{
		Model: gorm.Model{ID: 1},
		Name:  "Tester",
	}

	realm := &database.Realm{
		Model: gorm.Model{ID: 1},
		Name:  "Realmy",
	}

	cases := []struct {
		name        string
		user        *database.User
		realm       *database.Realm
		memberships []*database.Membership
		found       bool
		code        int
	}{
		{
			name: "no_user",
			user: nil,
			code: http.StatusInternalServerError,
		},
		{
			name:  "no_realm",
			user:  user,
			realm: nil,
			code:  http.StatusOK,
		},
		{
			name:        "no_memberships",
			user:        user,
			realm:       realm,
			memberships: nil,
			code:        http.StatusOK,
		},
		{
			name:  "memberships_missing",
			user:  user,
			realm: realm,
			memberships: []*database.Membership{
				{RealmID: 2, Realm: nil},
			},
			code: http.StatusOK,
		},
		{
			name:  "memberships_found",
			user:  user,
			realm: realm,
			memberships: []*database.Membership{
				{RealmID: realm.ID, Realm: realm},
			},
			found: true,
			code:  http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			if tc.user != nil {
				ctx = controller.WithUser(ctx, tc.user)
			}
			if tc.memberships != nil {
				ctx = controller.WithMemberships(ctx, tc.memberships)
			}

			session := &sessions.Session{
				Values: map[interface{}]interface{}{},
			}
			if tc.realm != nil {
				controller.StoreSessionRealm(session, tc.realm)
			}
			ctx = controller.WithSession(ctx, session)

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Accept", "application/json")

			w := httptest.NewRecorder()

			loadCurrentMembership(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()

				if tc.found {
					if m := controller.MembershipFromContext(ctx); m == nil {
						t.Errorf("expected membership in context")
					}
				} else {
					if m := controller.MembershipFromContext(ctx); m != nil {
						t.Errorf("expected no membership in context")
					}

					if id := controller.RealmIDFromSession(session); id != 0 {
						t.Errorf("expected realm to be cleared from session")
					}
				}
			})).ServeHTTP(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}

func TestRequireMembership(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	h, err := render.New(ctx, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	requireMembership := middleware.RequireMembership(h)

	cases := []struct {
		name       string
		membership *database.Membership
		code       int
	}{
		{
			name:       "missing",
			membership: nil,
			code:       http.StatusBadRequest,
		},
		{
			name: "default",
			membership: &database.Membership{
				User:  &database.User{},
				Realm: &database.Realm{},
			},
			code: http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			if tc.membership != nil {
				ctx = controller.WithMembership(ctx, tc.membership)
			}

			session := &sessions.Session{}
			ctx = controller.WithSession(ctx, session)

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.Clone(ctx)
			r.Header.Set("Accept", "application/json")

			w := httptest.NewRecorder()

			requireMembership(emptyHandler()).ServeHTTP(w, r)

			if got, want := w.Code, tc.code; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
		})
	}
}
