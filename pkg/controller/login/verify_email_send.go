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

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
)

func (c *Controller) HandleShowVerifyEmail() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}

		// Mark prompted so we only prompt once.
		controller.StoreSessionEmailVerificationPrompted(session, true)

		c.renderEmailVerify(ctx, w)
	})
}

func (c *Controller) HandleSubmitVerifyEmail() http.Handler {
	type FormData struct {
		IDToken string `form:"idToken"`
	}

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
		currentUser := membership.User

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Failed to verify email: %v", err)
			c.renderEmailVerify(ctx, w)
			return
		}

		// Build the email template.
		verifyComposer, err := controller.SendEmailVerificationEmailFunc(ctx, c.db, c.h, currentUser.Email, membership.Realm)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Send an email to verify the users email address.
		if err := c.authProvider.SendEmailVerificationEmail(ctx, currentUser.Email, form.IDToken, verifyComposer); err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		flash.Alert("Verification email sent.")
		c.renderEmailVerify(ctx, w)
	})
}

func (c *Controller) renderEmailVerify(ctx context.Context, w http.ResponseWriter) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Verify email address")
	m["firebase"] = c.config.Firebase
	c.h.RenderHTML(w, "login/verify-email", m)
}
