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
	"fmt"
	"net/http"
	"time"
	"unicode"

	"github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/flash"
)

func (c *Controller) HandleShowSelectNewPassword() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := controller.SessionFromContext(ctx)
		f := flash.New(session.Values)

		code := r.FormValue("oobCode")
		if code == "" {
			f.Error("No oobCode.")
			c.renderShowSelectPassword(ctx, w, "", code, f)
			return
		}

		email, err := c.firebaseInternal.VerifyPasswordResetCode(ctx, code)
		if err != nil {
			if errors.Is(err, firebase.ErrInvalidOOBCode) || errors.Is(err, firebase.ErrExpiredOOBCode) {
				f.Error("The action code is invalid. This can happen if the code is malformed, expired, or has already been used.")
			} else {
				f.Error("Error checking code. %v", err)
			}
			c.renderShowSelectPassword(ctx, w, "", code, f)
			return
		}

		c.renderShowSelectPassword(ctx, w, email, code, f)
	})
}

func (c *Controller) renderShowSelectPassword(ctx context.Context, w http.ResponseWriter, email, code string, f *flash.Flash) {
	m := controller.TemplateMapFromContext(ctx)
	m["email"] = email
	m["code"] = code
	m["flash"] = f
	m["requirements"] = &c.config.PasswordRequirements
	c.h.RenderHTML(w, "login/select-password", m)
}

func (c *Controller) HandleSubmitNewPassword() http.Handler {
	logger := c.logger.Named("login.HandleSubmitNewPassword")

	type FormData struct {
		Password string `form:"password"`
		Email    string `form:"email"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session := controller.SessionFromContext(ctx)
		f := flash.New(session.Values)

		code := r.FormValue("oobCode")

		var form FormData
		if err := controller.BindForm(w, r, &form); err != nil {
			f.Error("Select password failed. %v", err)
			c.renderShowSelectPassword(ctx, w, "", code, f)
			return
		}

		if err := c.validateComplexity(form.Password); err != nil {
			f.Error("Select password failed. %v", err)
			c.renderShowSelectPassword(ctx, w, form.Email, code, f)
			return
		}

		if _, err := c.firebaseInternal.ChangePasswordWithCode(ctx, code, form.Password); err != nil {
			if errors.Is(err, firebase.ErrInvalidOOBCode) || errors.Is(err, firebase.ErrExpiredOOBCode) {
				f.Error("The action code is invalid. This can happen if the code is malformed, expired, or has already been used.")
			} else {
				f.Error("Select password failed. %v", err)
			}
			c.renderShowSelectPassword(ctx, w, form.Email, code, f)
			return
		}

		if err := c.db.PasswordChanged(form.Email, time.Now()); err != nil {
			logger.Errorw("failed to mark password change time", "error", err)
			c.renderShowSelectPassword(ctx, w, form.Email, code, f)
			return
		}

		f.Alert("Successfully selected new password.")
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
