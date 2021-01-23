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

package smskeys

import (
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/rbac"
)

func (c *Controller) HandleDisable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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
		if !membership.Can(rbac.SettingsWrite) {
			controller.Unauthorized(w, r, c.h)
			return
		}
		currentRealm := membership.Realm
		currentUser := membership.User

		if !currentRealm.UseAuthenticatedSMS {
			currentRealm.AddError("", "Authenticated SMS is not currently enabled")
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShow(ctx, w, r, currentRealm)
			return
		}

		currentRealm.UseAuthenticatedSMS = false
		if err := c.db.SaveRealm(currentRealm, currentUser); err != nil {
			if database.IsNotFound(err) || database.IsValidationError(err) {
				currentRealm.AddError("", err.Error())
				w.WriteHeader(http.StatusUnprocessableEntity)
				c.renderShow(ctx, w, r, currentRealm)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Successfully disabled Authenticated SMS.")
		c.redirectShow(ctx, w, r)
	})
}
