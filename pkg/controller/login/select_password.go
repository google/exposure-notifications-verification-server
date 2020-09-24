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
	"time"
	"unicode"

	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleShowSelectNewPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c.renderShowSelectPassword(ctx, w)
	})
}

func (c *Controller) renderShowSelectPassword(ctx context.Context, w http.ResponseWriter) {
	m := controller.TemplateMapFromContext(ctx)
	m["firebase"] = c.config.Firebase
	m["requirements"] = &c.config.PasswordRequirements
	c.h.RenderHTML(w, "login/select-password", m)
}

func (c *Controller) HandleSubmitNewPassword() http.Handler {
	logger := c.logger.Named("login.HandleSubmitNewPassword")

	type FormData struct {
		Email    string `form:"email"`
		Password string `form:"password"`
		Code     string `form:"code"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			logger.Errorw("failed to bind form", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		m := controller.TemplateMapFromContext(ctx)
		f := flash.New(nil)

		if err := c.validateComplexity(form.Password); err != nil {
			logger.Errorw("validateComplexity failed", "error", err)
			f.Error("Select password failed. %v", err)
			c.renderShowSelectPassword(ctx, w)
			return
		}

		if _, err := c.fbInternal.VerifyPasswordResetCode(ctx, form.Code, form.Password); err != nil {
			logger.Errorw("VerifyPasswordResetCode failed", "error", err)
			c.renderShowSelectPassword(ctx, w)
			return
		}

		if err := c.db.PasswordChanged(form.Email, time.Now()); err != nil {
			logger.Errorw("failed to mark password change time", "error", err)
			controller.InternalError(w, r, c.h, err)
			return
		}

		// There's no session yet, so make a one-time flash.
		f.Alert("Successfully selected new password.")
		m["flash"] = f

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
