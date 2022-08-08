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

package controller

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-verification-server/internal/auth"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/render"
)

// SendInviteEmailFunc returns a function capable of sending a new user invitation.
func SendInviteEmailFunc(ctx context.Context, db *database.Database, h *render.Renderer, email string,
	realm *database.Realm,
) (auth.InviteUserEmailFunc, error) {
	// Lookup the email provider
	emailer, err := realm.EmailProvider(db)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to create email provider: %w", err)
	}

	// Return a function that does the actual sending.
	return func(ctx context.Context, inviteLink string) error {
		var message []byte
		if realm.EmailInviteTemplate != "" {
			// Render from the realm template with the plain header.
			header, err := h.RenderEmail("email/plainheader", map[string]interface{}{
				"ToEmail":   email,
				"FromEmail": emailer.From(),
			})
			if err != nil {
				return fmt.Errorf("failed to render email header template: %w", err)
			}
			body := []byte(realm.BuildInviteEmail(inviteLink))
			message = append(header, body...)
		} else {
			// Render the message invitation from the default template.
			message, err = h.RenderEmail("email/invite", map[string]interface{}{
				"ToEmail":    email,
				"FromEmail":  emailer.From(),
				"InviteLink": inviteLink,
				"RealmName":  realm.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to render invite template: %w", err)
			}
		}

		// Send the message.
		if err := emailer.SendEmail(ctx, email, message); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	}, nil
}

// SendPasswordResetEmailFunc returns a function capable of sending a password
// reset for the given user.
func SendPasswordResetEmailFunc(ctx context.Context, db *database.Database, h *render.Renderer, email string,
	realm *database.Realm,
) (auth.ResetPasswordEmailFunc, error) {
	// Lookup the email provider
	emailer, err := realm.EmailProvider(db)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to create email provider: %w", err)
	}

	return func(ctx context.Context, resetLink string) error {
		var message []byte
		if realm.EmailPasswordResetTemplate != "" {
			// Render from the realm template with the plain header.
			header, err := h.RenderEmail("email/plainheader", map[string]interface{}{
				"ToEmail":   email,
				"FromEmail": emailer.From(),
			})
			if err != nil {
				return fmt.Errorf("failed to render email header template: %w", err)
			}
			body := []byte(realm.BuildPasswordResetEmail(resetLink))
			message = append(header, body...)
		} else {
			// Render the reset email.
			message, err = h.RenderEmail("email/passwordresetemail", map[string]interface{}{
				"ToEmail":   email,
				"FromEmail": emailer.From(),
				"ResetLink": resetLink,
				"RealmName": realm.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to render password reset template: %w", err)
			}
		}

		// Send the message.
		if err := emailer.SendEmail(ctx, email, message); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	}, nil
}

// SendEmailVerificationEmailFunc returns a function capable of sending an email
// verification email.
func SendEmailVerificationEmailFunc(ctx context.Context, db *database.Database, h *render.Renderer, email string,
	realm *database.Realm,
) (auth.EmailVerificationEmailFunc, error) {
	// Lookup the email provider
	emailer, err := realm.EmailProvider(db)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to create email provider: %w", err)
	}

	return func(ctx context.Context, verifyLink string) error {
		var message []byte
		if realm.EmailVerifyTemplate != "" {
			// Render from the realm template with the plain header.
			header, err := h.RenderEmail("email/plainheader", map[string]interface{}{
				"ToEmail":   email,
				"FromEmail": emailer.From(),
			})
			if err != nil {
				return fmt.Errorf("failed to render email header template: %w", err)
			}
			body := []byte(realm.BuildVerifyEmail(verifyLink))
			message = append(header, body...)
		} else {
			// Render the reset email.
			message, err = h.RenderEmail("email/verifyemail", map[string]interface{}{
				"ToEmail":    email,
				"FromEmail":  emailer.From(),
				"VerifyLink": verifyLink,
				"RealmName":  realm.Name,
			})
			if err != nil {
				return fmt.Errorf("failed to render password reset template: %w", err)
			}
		}

		// Send the message.
		if err := emailer.SendEmail(ctx, email, message); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
		return nil
	}, nil
}
