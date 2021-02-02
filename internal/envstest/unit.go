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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
)

// ExerciseSessionMissing tests that the proper response code and HTML error
// page are rendered with the context has no session.
func ExerciseSessionMissing(t *testing.T, h http.Handler) {
	t.Run("session_missing", func(t *testing.T) {
		t.Parallel()

		ctx := project.TestContext(t)

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "session missing in request context"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
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

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 303; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "/login/select-realm"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
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

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "user missing"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
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

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Unauthorized"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
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

		r := httptest.NewRequest("GET", "/1", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		q := r.URL.Query()
		q.Set(pagination.QueryKeyPage, "banana")
		r.URL.RawQuery = q.Encode()

		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 400; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Bad request"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})
}

// ExerciseIDNotFound tests that the proper response code and HTML error page
// are rendered when the route expects an "id" mux parameter, but the one given
// does not correspond to an actual record.
func ExerciseIDNotFound(t *testing.T, membership *database.Membership, h http.Handler) {
	t.Run("id_not_found", func(t *testing.T) {
		t.Parallel()

		mux := mux.NewRouter()
		mux.Handle("/{id}", h)

		ctx := project.TestContext(t)
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, membership)
		ctx = controller.WithUser(ctx, membership.User)

		r := httptest.NewRequest("GET", "/13940890", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 401; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Unauthorized"; !strings.Contains(got, want) {
			t.Errorf("expected %q to contain %q", got, want)
		}
	})
}
