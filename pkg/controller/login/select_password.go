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
	"net/http"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleSelectNewPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		m := controller.TemplateMapFromContext(ctx)
		m["firebase"] = c.config.Firebase
		m["requirements"] = &c.config.PasswordRequirements
		c.h.RenderHTML(w, "login/select-password", m)
	})
}

func (c *Controller) HandleSubmitNewPassword() http.Handler {
	logger := c.logger.Named("HandleSubmitNewPassword")

	type FormData struct {
		Email string `form:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			logger.Errorw("failed to bind form", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		if err := c.db.PasswordChanged(form.Email, time.Now()); err != nil {
			logger.Errorw("failed to mark password change time", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		// There's no session yet, so make a one-time flash.
		m := controller.TemplateMapFromContext(ctx)
		f := flash.New(nil)
		f.Alert("New password selected successfully.")
		m["flash"] = f

		c.renderLogin(ctx, w)
	})
}
