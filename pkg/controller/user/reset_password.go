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

package user

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
	"github.com/gorilla/mux"
	"go.opencensus.io/stats"
)

func (c *Controller) HandleResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		membership := controller.MembershipFromContext(ctx)
		if membership == nil {
			controller.MissingMembership(w, r, c.h)
			return
		}
		if !membership.Can(rbac.UserWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}

		currentRealm := membership.Realm

		// Pull the user from the id.
		user, err := currentRealm.FindUser(c.db, vars["id"])
		if err != nil {
			if database.IsNotFound(err) {
				controller.Unauthorized(w, r, c.h)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// Ensure the upstream user exists. We have seen upstream users disappear in
		// certain auth providers if they don't log on for some period of time after
		// being invited to the system.
		if created, _ := c.authProvider.CreateUser(ctx, user.Name, user.Email, "", false, nil); created {
			stats.Record(ctx, mUpstreamUserRecreates.M(1))
		}

		// Build the emailer.
		resetComposer, err := controller.SendPasswordResetEmailFunc(ctx, c.db, c.h, user.Email, currentRealm)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Reset the password.
		if err := c.authProvider.SendResetPasswordEmail(ctx, user.Email, resetComposer); err != nil {
			flash.Error("Failed to reset password: %v", err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Successfully sent password reset to %v", user.Email)
		controller.Back(w, r, c.h)
		return
	})
}
