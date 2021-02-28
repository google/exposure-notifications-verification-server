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

package envstest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// ExerciseSessionMissing tests that the proper response code and HTML error
// page are rendered with the context has no session.
func ExerciseSessionMissing(t *testing.T, h http.Handler) {
	t.Run("session_missing", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "session missing in request context"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}

// ExerciseMembershipMissing tests that the proper response code and HTML error
// page are rendered with there is no membership in the context. It sets a
// session in the context.
func ExerciseMembershipMissing(t *testing.T, h http.Handler) {
	t.Run("membership_missing", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusSeeOther; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "/login/select-realm"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}

// ExerciseUserMissing tests that the proper response code and HTML error page
// are rendered with there is no user in the context. It sets a session in the
// context. This only applies to admin pages
func ExerciseUserMissing(t *testing.T, h http.Handler) {
	t.Run("user_missing", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusInternalServerError; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "user missing"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}

// ExercisePermissionMissing tests that the proper response code and HTML error
// page are rendered when the requestor does not have permission to perform this
// action.
func ExercisePermissionMissing(t *testing.T, h http.Handler) {
	t.Run("permission_missing", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{})
		ctx = controller.WithUser(ctx, &database.User{})

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Unauthorized"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}

// ExerciseBadPagination tests that the proper response code and HTML error page
// are rendered when the URL includes pagination parameters that fail to parse.
func ExerciseBadPagination(t *testing.T, membership *database.Membership, h http.Handler) {
	t.Run("bad_pagination", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, membership)
		ctx = controller.WithUser(ctx, membership.User)

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/1?page=banana", nil)
		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, http.StatusBadRequest; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Bad request"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}

// ExerciseIDNotFound tests that the proper response code and HTML error page
// are rendered when the route expects an "id" mux parameter, but the one given
// does not correspond to an actual record.
func ExerciseIDNotFound(t *testing.T, membership *database.Membership, h http.Handler) {
	t.Run("id_not_found", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, membership)
		ctx = controller.WithUser(ctx, membership.User)

		w, r := BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
		r = mux.SetURLVars(r, map[string]string{"id": "13940890"})
		h.ServeHTTP(w, r)

		if got, want := w.Code, http.StatusUnauthorized; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Unauthorized"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}
