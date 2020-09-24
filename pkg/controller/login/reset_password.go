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

	"github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleShowResetPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderResetPassword(ctx, w, nil)
	})
}

func (c *Controller) renderResetPassword(ctx context.Context, w http.ResponseWriter, f *flash.Flash) {
	m := controller.TemplateMapFromContext(ctx)
	m["flash"] = f
	c.h.RenderHTML(w, "login/reset-password", m)
}

func (c *Controller) HandleSubmitResetPassword() http.Handler {
	type FormData struct {
		Email string `form:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := controller.SessionFromContext(ctx)
		f := flash.New(session.Values)

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			f.Error("Password reset failed. %v", err)
			c.renderResetPassword(ctx, w, f)
			return
		}

		if err := c.firebaseInternal.SendPasswordResetEmail(ctx, form.Email); err != nil {
			// Treat not-found like success so we don't leak details.
			if !errors.Is(err, firebase.ErrEmailNotFound) {
				f.Error("Password reset failed.")
				c.renderResetPassword(ctx, w, f)
				return
			}
		}

		f.Alert("Password reset email sent.")
		c.renderResetPassword(ctx, w, f)
	})
}
