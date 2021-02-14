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

package user_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/envstest"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	userpkg "github.com/google/exposure-notifications-verification-server/pkg/controller/user"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

func TestHandleExport(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServer(t, testDatabaseInstance)

	realm, admin, _, err := harness.ProvisionAndLogin()
	if err != nil {
		t.Fatal(err)
	}

	// Create another user.
	user := &database.User{
		Email: "user@example.com",
		Name:  "User",
	}
	if err := harness.Database.SaveUser(user, database.SystemTest); err != nil {
		t.Fatal(err)
	}
	if err := user.AddToRealm(harness.Database, realm, rbac.LegacyRealmAdmin, database.SystemTest); err != nil {
		t.Fatal(err)
	}

	h, err := render.New(ctx, envstest.ServerAssetsPath(), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)
		handler := c.HandleExport()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
		envstest.ExercisePermissionMissing(t, handler)
	})

	t.Run("internal_error", func(t *testing.T) {
		t.Parallel()

		harness := envstest.NewServerConfig(t, testDatabaseInstance)
		harness.Database.SetRawDB(envstest.NewFailingDatabase())

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/", c.HandleExport()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.UserRead,
		})

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 500; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}
		if got, want := w.Body.String(), "Internal server error"; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})

	t.Run("csvs", func(t *testing.T) {
		t.Parallel()

		c := userpkg.New(harness.AuthProvider, harness.Cacher, harness.Database, h)

		mux := mux.NewRouter()
		mux.Handle("/", c.HandleExport()).Methods("GET")

		ctx := ctx
		ctx = controller.WithSession(ctx, &sessions.Session{})
		ctx = controller.WithMembership(ctx, &database.Membership{
			Realm:       realm,
			User:        admin,
			Permissions: rbac.UserRead,
		})

		r := httptest.NewRequest("GET", "/", nil)
		r = r.Clone(ctx)
		r.Header.Set("Content-Type", "text/html")

		w := httptest.NewRecorder()

		mux.ServeHTTP(w, r)
		w.Flush()

		if got, want := w.Code, 200; got != want {
			t.Errorf("Expected %d to be %d", got, want)
		}

		d := time.Now().UTC().Format(project.RFC3339Date)
		exp := fmt.Sprintf(`name,email,joined
System admin,super@example.com,%s
User,user@example.com,%s
`, d, d)
		if got, want := w.Body.String(), exp; !strings.Contains(got, want) {
			t.Errorf("Expected %q to contain %q", got, want)
		}
	})
}
