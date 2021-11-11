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
	"fmt"
	"net/http"
	"time"
	"unicode"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleShowSelectNewPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := controller.SessionFromContext(ctx)
		flash := flash.New(session.Values)

		code := r.FormValue("oobCode")
		if code == "" {
			flash.Error("Missing password reset token.")
			c.renderShowSelectPassword(ctx, w, "", code, true)
			return
		}

		email, err := c.authProvider.VerifyPasswordResetCode(ctx, code)
		if err != nil {
			flash.Error("Failed to verify password reset token: %v", err)
			c.renderShowSelectPassword(ctx, w, "", code, true)
			return
		}

		c.renderShowSelectPassword(ctx, w, email, code, false)
	})
}

func (c *Controller) renderShowSelectPassword(ctx context.Context, w http.ResponseWriter, email, code string, codeInvalid bool) {
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Change password")
	m["email"] = email
	m["code"] = code
	m["codeInvalid"] = codeInvalid
	m["requirements"] = &c.config.PasswordRequirements
	c.h.RenderHTML(w, "login/select-password", m)
}

func (c *Controller) HandleSubmitNewPassword() http.Handler {
	type FormData struct {
		Password string `form:"password"`
		Email    string `form:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("login.HandleSubmitNewPassword")

		session := controller.SessionFromContext(ctx)
		if session == nil {
			controller.MissingSession(w, r, c.h)
			return
		}
		flash := controller.Flash(session)

		code := r.FormValue("oobCode")

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			flash.Error("Select password failed: %v", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			c.renderShowSelectPassword(ctx, w, "", code, false)
			return
		}
		email := project.TrimSpace(form.Email)

		if err := c.validateComplexity(form.Password); err != nil {
			flash.Error("Select password failed: %v", err)
			c.renderShowSelectPassword(ctx, w, email, code, false)
			return
		}

		if err := c.authProvider.ChangePassword(ctx, form.Password, code); err != nil {
			flash.Error("Failed to change password: %v", err)
			c.renderShowSelectPassword(ctx, w, email, code, true)
			return
		}

		if err := c.db.PasswordChanged(email, time.Now().UTC()); err != nil {
			logger.Errorw("failed to mark password change time", "error", err)
		}

		flash.Alert("Successfully selected new password.")
		c.renderLogin(ctx, w)
	})
}

func (c *Controller) validateComplexity(password string) error {
	reqs := c.config.PasswordRequirements
	if len(password) < reqs.Length {
		return fmt.Errorf("password must be at least %d characters long", reqs.Length)
	}

	upperCount := 0
	lowerCount := 0
	digitCount := 0
	specialCount := 0
	for _, c := range password {
		if unicode.IsLetter(c) {
			if unicode.IsUpper(c) {
				upperCount++
			} else {
				lowerCount++
			}
		} else if unicode.IsDigit(c) {
			digitCount++
		} else {
			specialCount++
		}
	}

	if upperCount < reqs.Uppercase {
		return fmt.Errorf("password must contain at least %d uppercase characters", reqs.Uppercase)
	}
	if lowerCount < reqs.Lowercase {
		return fmt.Errorf("password must contain at least %d lowercase characters", reqs.Lowercase)
	}
	if digitCount < reqs.Number {
		return fmt.Errorf("password must contain at least %d digits", reqs.Number)
	}
	if specialCount < reqs.Special {
		return fmt.Errorf("password must contain at least %d special characters", reqs.Number)
	}

	return nil
}
