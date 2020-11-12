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

package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gorilla/sessions"
)

const (
	sessionKeyLocalCookie = sessionKey("localCookie")
)

type localAuth struct{}

// NewLocal creates a new auth provider for local auth.
func NewLocal(ctx context.Context) (Provider, error) {
	return &localAuth{}, nil
}

// CheckRevoked checks if the users auth has been revoked.
func (a *localAuth) CheckRevoked(ctx context.Context, session *sessions.Session) error {
	data, err := a.loadCookie(ctx, session)
	if err != nil {
		return err
	}

	if data.Revoked {
		return fmt.Errorf("session is revoked")
	}
	return nil
}

// StoreSession stores information about the session.
func (a *localAuth) StoreSession(ctx context.Context, session *sessions.Session, i *SessionInfo) error {
	if i == nil || i.Data == nil {
		a.ClearSession(ctx, session)
		return ErrSessionInfoMissing
	}

	email, ok := i.Data["email"].(string)
	if !ok {
		a.ClearSession(ctx, session)
		return fmt.Errorf("missing email: %w", ErrSessionInfoMissing)
	}

	emailVerified, ok := i.Data["email_verified"].(bool)
	if !ok {
		a.ClearSession(ctx, session)
		return fmt.Errorf("missing email_verified: %w", ErrSessionInfoMissing)
	}

	mfaEnabled, ok := i.Data["mfa_enabled"].(bool)
	if !ok {
		a.ClearSession(ctx, session)
		return fmt.Errorf("missing mfa_enabled: %w", ErrSessionInfoMissing)
	}

	revoked, ok := i.Data["revoked"].(bool)
	if !ok {
		a.ClearSession(ctx, session)
		return fmt.Errorf("missing revoked: %w", ErrSessionInfoMissing)
	}

	// Convert ID token to long-lived cookie
	cookie, err := json.Marshal(&localCookieData{
		Email:         email,
		EmailVerified: emailVerified,
		MFAEnabled:    mfaEnabled,
		Revoked:       revoked,
	})
	if err != nil {
		a.ClearSession(ctx, session)
		return err
	}

	// Set cookie
	if err := sessionSet(session, sessionKeyLocalCookie, string(cookie)); err != nil {
		a.ClearSession(ctx, session)
		return err
	}

	return nil
}

// ClearSession removes any session information for this auth.
func (a *localAuth) ClearSession(ctx context.Context, session *sessions.Session) {
	sessionClear(session, sessionKeyLocalCookie)
}

// CreateUser creates a user in the upstream auth system with the given name and
// email. It returns true if the user was created or false if the user already
// exists.
func (a *localAuth) CreateUser(ctx context.Context, name, email, pass string, sendInvite bool, emailer InviteUserEmailFunc) (bool, error) {
	if !sendInvite {
		return true, nil
	}

	if emailer == nil {
		return true, nil
	}

	// For local auth, this is a noop since the controllers create the user in the
	// database.

	// Send the welcome email.
	inviteLink, err := a.passwordResetLink(ctx, email)
	if err != nil {
		return true, err
	}

	if err := emailer(ctx, inviteLink); err != nil {
		return true, fmt.Errorf("failed to send new user invitation email: %w", err)
	}

	return true, nil
}

// EmailAddress extracts the users email from the session.
func (a *localAuth) EmailAddress(ctx context.Context, session *sessions.Session) (string, error) {
	data, err := a.loadCookie(ctx, session)
	if err != nil {
		return "", err
	}
	return data.Email, nil
}

// EmailVerified returns true if the current user is verified, false otherwise.
func (a *localAuth) EmailVerified(ctx context.Context, session *sessions.Session) (bool, error) {
	data, err := a.loadCookie(ctx, session)
	if err != nil {
		return false, err
	}
	return data.EmailVerified, nil
}

// MFAEnabled returns whether MFA is enabled on the account.
func (a *localAuth) MFAEnabled(ctx context.Context, session *sessions.Session) (bool, error) {
	data, err := a.loadCookie(ctx, session)
	if err != nil {
		return false, err
	}
	return data.MFAEnabled, nil
}

// ChangePassword changes the users password. The data is not used. Since local
// auth does not use passwords, this is a noop.
func (a *localAuth) ChangePassword(ctx context.Context, newPassword string, data interface{}) error {
	return nil
}

// SendResetPasswordEmail resets the password for the given user. If the user does not
// exist, an error is returned.
func (a *localAuth) SendResetPasswordEmail(ctx context.Context, email string, emailer ResetPasswordEmailFunc) error {
	if emailer == nil {
		return fmt.Errorf("emailer is required for local auth")
	}

	resetLink, err := a.passwordResetLink(ctx, email)
	if err != nil {
		return err
	}

	if err := emailer(ctx, resetLink); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}

// VerifyPasswordResetCode does nothing. It returns the empty string.
func (a *localAuth) VerifyPasswordResetCode(ctx context.Context, code string) (string, error) {
	return "", nil
}

// SendEmailVerificationEmail sends an message to the currently authenticated
// user, asking them to verify ownership of the email address.
func (a *localAuth) SendEmailVerificationEmail(ctx context.Context, email string, data interface{}, emailer EmailVerificationEmailFunc) error {
	if emailer == nil {
		return fmt.Errorf("emailer is required for local auth")
	}

	verifyLink, err := a.emailVerificationLink(ctx, email)
	if err != nil {
		return err
	}

	if err := emailer(ctx, verifyLink); err != nil {
		return fmt.Errorf("failed to send email verification email: %w", err)
	}

	return nil
}

// passwordResetLink generates and returns the password reset link for the given
// email (user).
func (a *localAuth) passwordResetLink(ctx context.Context, email string) (string, error) {
	return "", fmt.Errorf("not yet implemented for local auth")
}

// emailVerificationLink generates an email verification link for the given
// email.
func (a *localAuth) emailVerificationLink(ctx context.Context, email string) (string, error) {
	return "", fmt.Errorf("not yet implemented for local auth")
}

type localCookieData struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	MFAEnabled    bool   `json:"mfa_enabled"`
	Revoked       bool   `json:"revoked"`
}

// dataFromCookie extracts the information from the provided local cookie, if it
// exists. The local cookie is actually just a JSON payload.
func (a *localAuth) dataFromCookie(ctx context.Context, cookie string) (*localCookieData, error) {
	var data localCookieData
	if err := json.Unmarshal([]byte(cookie), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// loadCookie loads and parses the local cookie from the session.
func (a *localAuth) loadCookie(ctx context.Context, session *sessions.Session) (*localCookieData, error) {
	raw, err := sessionGet(session, sessionKeyLocalCookie)
	if err != nil {
		a.ClearSession(ctx, session)
		return nil, err
	}

	cookie, ok := raw.(string)
	if !ok || cookie == "" {
		a.ClearSession(ctx, session)
		return nil, ErrSessionMissing
	}

	data, err := a.dataFromCookie(ctx, cookie)
	if err != nil {
		a.ClearSession(ctx, session)
		return nil, err
	}
	return data, nil
}
