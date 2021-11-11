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

// Package login defines the controller for the login page.
package login

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

func (c *Controller) HandleShowResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderResetPassword(ctx, w, "")
	})
}

func (c *Controller) renderResetPassword(ctx context.Context, w http.ResponseWriter, email string) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Reset password")
	m["email"] = email
	c.h.RenderHTML(w, "login/reset-password", m)
}

func (c *Controller) HandleSubmitResetPassword() http.Handler {
	type FormData struct {
		Email string `form:"email,required"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("HandleSubmitResetPassword")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to reset password: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderResetPassword(ctx, w, "")
			return
		}

		// Does the user exist?
		user, err := c.db.FindUserByEmail(form.Email)
		if err != nil {
			if database.IsNotFound(err) {
				// Fake success - we don't want to reveal if this is a user
				// of our system from an unauthorized context.
				flash.Error("Password reset email sent.")
				c.renderResetPassword(ctx, w, form.Email)
				return
			}

			controller.InternalError(w, r, c.h, err)
			return
		}

		// nil composer falls back to firebase and no custom message.
		var resetComposer auth.ResetPasswordEmailFunc

		membership := controller.MembershipFromContext(ctx)

		// This is likely - most users reset password from un-authed context.
		if membership == nil {
			// Use the first membership available. This will help get the user a realm-localized template.
			// For most users the expected number of memberships is 1.
			membership, err = user.SelectFirstMembership(c.db)
			if err != nil {
				if database.IsNotFound(err) {
					logger.Infof("No membership found for %s", user.Email)
				} else {
					controller.InternalError(w, r, c.h, err)
					return
				}
			}
		}

		// Build the emailer.
		if membership != nil {
			resetComposer, err = controller.SendPasswordResetEmailFunc(ctx, c.db, c.h, user.Email, membership.Realm)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
		}

		// Reset the password.
		if err := c.authProvider.SendResetPasswordEmail(ctx, user.Email, resetComposer); err != nil {
			flash.Error("Failed to reset password: %v", err)
			controller.Back(w, r, c.h)
			return
		}

		flash.Alert("Password reset email sent.")
		c.renderResetPassword(ctx, w, user.Email)
	})
}
