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
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/sessions"
)

func TestHandlePostAuthenticate(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	harness := envstest.NewServerConfig(t, testDatabaseInstance)

	c := login.New(harness.AuthProvider, harness.Cacher, harness.Config, harness.Database, harness.Renderer)
	handler := c.HandlePostAuthenticate()

	t.Run("middleware", func(t *testing.T) {
		t.Parallel()

		envstest.ExerciseSessionMissing(t, handler)
		envstest.ExerciseMembershipMissing(t, handler)
	})

	cases := []struct {
		name  string
		perms rbac.Permission
		exp   string
	}{
		{
			name:  "code_issue",
			perms: rbac.CodeIssue,
			exp:   "/codes/issue",
		},
		{
			name:  "code_issue_priority",
			perms: rbac.CodeIssue | rbac.SettingsRead,
			exp:   "/codes/issue",
		},
		{
			name:  "code_issue_fallback",
			perms: 0,
			exp:   "/codes/issue",
		},
		{
			name:  "bulk_issue",
			perms: rbac.CodeBulkIssue,
			exp:   "/codes/bulk-issue",
		},
		{
			name:  "stats_read",
			perms: rbac.StatsRead,
			exp:   "/realm/stats",
		},
		{
			name:  "settings_read",
			perms: rbac.SettingsRead,
			exp:   "/realm/settings",
		},
		{
			name:  "audit_read",
			perms: rbac.AuditRead,
			exp:   "/realm/events",
		},
		{
			name:  "user_read",
			perms: rbac.UserRead,
			exp:   "/realm/users",
		},
		{
			name:  "apikey_read",
			perms: rbac.APIKeyRead,
			exp:   "/realm/apikeys",
		},
		{
			name:  "mobile_apps_read",
			perms: rbac.MobileAppRead,
			exp:   "/realm/mobile-apps",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := ctx
			ctx = controller.WithSession(ctx, &sessions.Session{})
			ctx = controller.WithMembership(ctx, &database.Membership{
				Realm:       &database.Realm{},
				User:        &database.User{},
				Permissions: tc.perms,
			})

			w, r := envstest.BuildFormRequest(ctx, t, http.MethodGet, "/", nil)
			handler.ServeHTTP(w, r)

			if got, want := w.Code, http.StatusSeeOther; got != want {
				t.Errorf("Expected %d to be %d", got, want)
			}
			if got, want := w.Header().Get("Location"), tc.exp; !strings.Contains(got, want) {
				t.Errorf("Expected %q to contain %q", got, want)
			}
		})
	}
}
