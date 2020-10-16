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
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"go.opencensus.io/stats"
)

func (c *Controller) HandleShowResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderResetPassword(ctx, w, nil, "", false)
	})
}

func (c *Controller) renderResetPassword(
	ctx context.Context, w http.ResponseWriter,
	flash *flash.Flash, email string, reset bool) {
	m := controller.TemplateMapFromContext(ctx)
	m["flash"] = flash
	m["email"] = email
	if reset {
		m["firebase"] = c.config.Firebase
	}
	c.h.RenderHTML(w, "login/reset-password", m)
}

func (c *Controller) HandleSubmitResetPassword() http.Handler {
	type FormData struct {
		Email string `form:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := controller.SessionFromContext(ctx)
		flash := flash.New(session.Values)

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Password reset failed. %v", err)
			c.renderResetPassword(ctx, w, flash, "", false)
			return
		}
		email := strings.TrimSpace(form.Email)

		// Ensure that if we have a user, they have auth
		user, err := c.db.FindUserByEmail(email)
		if err != nil {
			if database.IsNotFound(err) {
				flash.Alert("Password reset email sent.")
			}
			c.renderResetPassword(ctx, w, flash, email, false)
			return
		}

		if created, _ := user.CreateFirebaseUser(ctx, c.client); created {
			stats.Record(ctx, controller.MFirebaseRecreates.M(1))
		}

		sent, err := c.sendResetFromSystemEmailer(ctx, email)
		if err != nil {
			c.logger.Warnw("failed sending password reset", "error", err)
		}
		if !sent {
			// fallback to firebase
			c.renderResetPassword(ctx, w, flash, email, true)
			return
		}

		flash.Alert("Password reset email sent.")
		c.renderResetPassword(ctx, w, flash, email, false)
	})
}

func (c *Controller) sendResetFromSystemEmailer(ctx context.Context, toEmail string) (bool, error) {
	// Send email with system email config

	emailConfig, err := c.db.SystemEmailConfig()
	if err != nil {
		if !database.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get email config for system: %w", err)
	}

	emailer, err := emailConfig.Provider()
	if err != nil {
		return false, fmt.Errorf("failed to get emailer for realm: %w", err)
	}

	message, err := controller.ComposePasswordResetEmail(ctx, c.h, c.client, toEmail, emailer.From())
	if err != nil {
		return false, fmt.Errorf("failed composing password reset email: %w", err)
	}

	if err := emailer.SendEmail(ctx, toEmail, message); err != nil {
		return false, fmt.Errorf("failed sending password reset email: %w", err)
	}

	return true, nil
}
