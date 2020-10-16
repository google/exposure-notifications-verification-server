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
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

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
