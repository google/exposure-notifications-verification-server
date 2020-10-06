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
	"errors"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
	"go.opencensus.io/stats"
)

func (c *Controller) HandleShowResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderResetPassword(ctx, w, nil)
	})
}

func (c *Controller) renderResetPassword(ctx context.Context, w http.ResponseWriter, flash *flash.Flash) {
	m := controller.TemplateMapFromContext(ctx)
	m["flash"] = flash
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
			c.renderResetPassword(ctx, w, flash)
			return
		}

		// Ensure that if we have a user, they have auth
		if user, err := c.db.FindUserByEmail(form.Email); err == nil {
			if created, _ := user.CreateFirebaseUser(ctx, c.client); created {
				stats.Record(ctx, controller.MFirebaseRecreates.M(1))
			}
		}

		if err := c.firebaseInternal.SendPasswordResetEmail(ctx, strings.TrimSpace(form.Email)); err != nil {
			if errors.Is(err, firebase.ErrTooManyAttempts) {
				flash.Error("Too many attempts have been made. Please wait and try again later.")
				c.renderResetPassword(ctx, w, flash)
				return
			}

			// Treat not-found like success so we don't leak details.
			if !errors.Is(err, firebase.ErrEmailNotFound) {
				flash.Error("Password reset failed.")
				c.renderResetPassword(ctx, w, flash)
				return
			}
		}

		flash.Alert("Password reset email sent.")
		c.renderResetPassword(ctx, w, flash)
	})
}
