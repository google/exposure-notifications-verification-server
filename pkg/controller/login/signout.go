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

package login

import (
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

// HandleSignOut handles session termination. It's possible for a user to
// navigate to this page directly (without auth).
func (c *Controller) HandleSignOut() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("login.HandleSignOut")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		// If a user is currently logged in, update their last revoked check time.
		email, err := c.authProvider.EmailAddress(ctx, session)
		if err != nil {
			logger.Debugw("failed to lookup user by email", "error", err)
		} else {
			currentUser, err := c.db.FindUserByEmail(email)
			if err != nil {
				if !database.IsNotFound(err) {
					controller.InternalError(w, r, c.h, err)
					return
				}
			}

			if currentUser != nil {
				// Update the user's last revoked checked time.
				if err := c.db.UntouchUserRevokeCheck(currentUser); err != nil {
					controller.InternalError(w, r, c.h, err)
					return
				}
			}
		}

		// Revoke upstream session.
		if err := c.authProvider.RevokeSession(ctx, session); err != nil {
			// This is just a warning since a user could navigate to /signout without
			// an active session.
			logger.Debugw("failed to revoke session", "error", err)
		}

		// Clear all session data, but copy over flashes.
		session.Values = make(map[interface{}]interface{})
		flash.Clone(session.Values)

		m := controller.TemplateMapFromContext(ctx)
		m.Title("Logging out...")
		m["firebase"] = c.config.Firebase
		c.h.RenderHTML(w, "signout", m)
	})
}
