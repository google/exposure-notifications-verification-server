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

package controller

import (
	"context"
	"fmt"

	"firebase.google.com/go/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/email"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

type ComposeFn func(provider email.Provider, realm *database.Realm, toEmail string) ([]byte, error)

// ComposeInviteEmail uses the renderer and auth client to generate a password reset link
// and emit an invite email
func ComposeInviteEmail(
	ctx context.Context,
	h *render.Renderer,
	auth *auth.Client, toEmail, fromEmail, realmName string) ([]byte, error) {
	inviteLink, err := auth.PasswordResetLink(ctx, toEmail)
	if err != nil {
		return nil, fmt.Errorf("failed generating reset link: %w", err)
	}

	// Compose message
	message, err := h.RenderEmail("email/invite",
		struct {
			ToEmail    string
			FromEmail  string
			InviteLink string
			RealmName  string
		}{
			ToEmail:    toEmail,
			FromEmail:  fromEmail,
			InviteLink: inviteLink,
			RealmName:  realmName,
		})
	if err != nil {
		return nil, fmt.Errorf("failed rendering invite template: %w", err)
	}
	return message, nil
}

// ComposeEmailVerifyEmail uses the renderer and auth client to generate an email to verify
// the user's email address.
func ComposeEmailVerifyEmail(
	ctx context.Context,
	h *render.Renderer,
	auth *auth.Client, toEmail, fromEmail, realmName string) ([]byte, error) {
	verifyLink, err := auth.EmailVerificationLink(ctx, toEmail)
	if err != nil {
		return nil, fmt.Errorf("failed generating verification link: %w", err)
	}

	// Compose message
	message, err := h.RenderEmail("email/verifyemail",
		struct {
			ToEmail    string
			FromEmail  string
			VerifyLink string
			RealmName  string
		}{
			ToEmail:    toEmail,
			FromEmail:  fromEmail,
			VerifyLink: verifyLink,
			RealmName:  realmName,
		})
	if err != nil {
		return nil, fmt.Errorf("failed rendering email verification template: %w", err)
	}
	return message, nil
}

// ComposePasswordResetEmail uses the renderer and auth client to generate an email for resetting
// a user password
func ComposePasswordResetEmail(
	ctx context.Context,
	h *render.Renderer,
	auth *auth.Client, toEmail, fromEmail string) ([]byte, error) {
	resetLink, err := auth.PasswordResetLink(ctx, toEmail)
	if err != nil {
		return nil, fmt.Errorf("failed generating password reset link: %w", err)
	}

	// Compose message
	message, err := h.RenderEmail("email/passwordresetemail",
		struct {
			ToEmail   string
			FromEmail string
			ResetLink string
			RealmName string
		}{
			ToEmail:   toEmail,
			FromEmail: fromEmail,
			ResetLink: resetLink,
		})
	if err != nil {
		return nil, fmt.Errorf("failed rendering password reset template: %w", err)
	}
	return message, nil
}

func SendRealmEmail(ctx context.Context, db *database.Database, compose ComposeFn, toEmail string) (bool, error) {
	realm := RealmFromContext(ctx)
	if realm == nil {
		return false, nil
	}

	emailer, err := realm.EmailProvider(db)
	if err != nil {
		if database.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed creating email provider: %w", err)
	}

	message, err := compose(emailer, realm, toEmail)
	if err != nil {
		return false, fmt.Errorf("failed composing email verification: %w", err)
	}

	if err := emailer.SendEmail(ctx, toEmail, message); err != nil {
		return false, fmt.Errorf("failed sending email verification: %w", err)
	}

	return true, nil
}
