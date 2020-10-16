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

// Package login defines the controller for the login page.
package login

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
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

		c.renderEmailVerify(ctx, w, "")
	})
}

func (c *Controller) HandleSubmitVerifyEmail() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		currentUser := controller.UserFromContext(ctx)
		if currentUser == nil {
			controller.MissingUser(w, r, c.h)
			return
		}

		sent, err := c.sendVerificationFromRealmEmailer(ctx, currentUser.Email)
		if err != nil {
			c.logger.Warnw("failed sending verification", "error", err)
		}
		if !sent {
			// fallback to firebase
			c.renderEmailVerify(ctx, w, "send")
			return
		}

		flash.Alert("Verification email sent.")
		c.renderEmailVerify(ctx, w, "sent")
	})
}

func (c *Controller) renderEmailVerify(ctx context.Context, w http.ResponseWriter, sendInvite string) {
	m := controller.TemplateMapFromContext(ctx)
	m["sendInvite"] = sendInvite
	m["firebase"] = c.config.Firebase
	c.h.RenderHTML(w, "login/verify-email", m)
}

// sendVerificationFromRealmEmailer send email with the realm email config
func (c *Controller) sendVerificationFromRealmEmailer(ctx context.Context, toEmail string) (bool, error) {
	realm := controller.RealmFromContext(ctx)
	if realm == nil {
		return false, nil
	}

	emailer, err := realm.EmailProvider(c.db)
	if err != nil {
		if !database.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed creating email provider: %w", err)
	}

	message, err := controller.ComposeInviteEmail(ctx, c.h, c.client, toEmail, emailer.From(), realm.Name)
	if err != nil {
		return false, fmt.Errorf("failed composing email verification: %w", err)
	}

	if err := emailer.SendEmail(ctx, toEmail, message); err != nil {
		return false, fmt.Errorf("failed sending email verification: %w", err)
	}

	return true, nil
}
