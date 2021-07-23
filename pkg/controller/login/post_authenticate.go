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

// Package login defines the controller for the login page.
package login

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandlePostAuthenticate() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		currentMembership := controller.MembershipFromContext(ctx)
		if currentMembership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}

		switch {
		case currentMembership.Can(rbac.CodeIssue):
			http.Redirect(w, r, "/codes/issue", http.StatusSeeOther)
		case currentMembership.Can(rbac.CodeBulkIssue):
			http.Redirect(w, r, "/codes/bulk-issue", http.StatusSeeOther)
		case currentMembership.Can(rbac.StatsRead):
			http.Redirect(w, r, "/realm/stats", http.StatusSeeOther)
		case currentMembership.Can(rbac.SettingsRead):
			http.Redirect(w, r, "/realm/settings", http.StatusSeeOther)
		case currentMembership.Can(rbac.AuditRead):
			http.Redirect(w, r, "/realm/events", http.StatusSeeOther)
		case currentMembership.Can(rbac.UserRead):
			http.Redirect(w, r, "/realm/users", http.StatusSeeOther)
		case currentMembership.Can(rbac.APIKeyRead):
			http.Redirect(w, r, "/realm/apikeys", http.StatusSeeOther)
		case currentMembership.Can(rbac.MobileAppRead):
			http.Redirect(w, r, "/realm/mobile-apps", http.StatusSeeOther)
		default:
			// The user probably has no RBAC permissions. Redirect to code issue
			// (which will fail), but preserves existing behavior.
			http.Redirect(w, r, "/codes/issue", http.StatusSeeOther)
		}
	})
}
