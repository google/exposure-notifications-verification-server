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

// Package auth exposes interfaces for various auth methods.
package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/sessions"
)

var (
	ErrSessionMissing     = fmt.Errorf("session is missing")
	ErrSessionInfoMissing = fmt.Errorf("session info is missing")
)

// InviteUserEmailFunc sends email with the given inviteLink.
type InviteUserEmailFunc func(ctx context.Context, inviteLink string) error

// ResetPasswordEmailFunc is an email composer function.
type ResetPasswordEmailFunc func(ctx context.Context, resetLink string) error

// EmailVerificationEmailFunc is an email composer function.
type EmailVerificationEmailFunc func(ctx context.Context, verifyLink string) error

// Provider is a generic authentication provider interface.
type Provider interface {
	// StoreSession stores the session in the values.
	StoreSession(context.Context, *sessions.Session, *SessionInfo) error

	// CheckRevoked checks if the auth has been revoked. It returns an error if
	// the auth does not exist or if the auth has been revoked.
	CheckRevoked(context.Context, *sessions.Session) error

	// ClearSession removes any information about this auth from the session.
	ClearSession(context.Context, *sessions.Session)

	// CreateUser creates a user in the auth provider. If pass is "", the provider
	// creates and uses a random password.
	CreateUser(ctx context.Context, name, email, pass string, sendInvite bool, composer InviteUserEmailFunc) (bool, error)

	// SendResetPasswordEmail resets the given user's password. If the user does not exist,
	// the underlying provider determines whether it's an error or perhaps upserts
	// the account.
	SendResetPasswordEmail(ctx context.Context, email string, composer ResetPasswordEmailFunc) error

	// ChangePassword changes the users password. The additional authentication
	// information is provider-specific.
	ChangePassword(ctx context.Context, newPassword string, data interface{}) error

	// VerifyPasswordResetCode verifies the code is valid. It returns the email of
	// the user for which the code belongs.
	VerifyPasswordResetCode(ctx context.Context, code string) (string, error)

	// SendEmailVerificationEmail sends the email verification email for the
	// currently authenticated user. Data is arbitrary additional data that the
	// provider might need (like user ID) to send the verification.
	SendEmailVerificationEmail(ctx context.Context, email string, data interface{}, composer EmailVerificationEmailFunc) error

	// EmailAddress extracts the email address for this auth provider from the
	// session. It returns an error if the session does not exist.
	EmailAddress(context.Context, *sessions.Session) (string, error)

	// EmailVerified returns true if the current user is verified, false
	// otherwise.
	EmailVerified(context.Context, *sessions.Session) (bool, error)

	// MFAEnabled returns true if MFA is enabled, false otherwise.
	MFAEnabled(context.Context, *sessions.Session) (bool, error)
}

// SessionInfo is a generic struct used to store session information. Not all
// providers use all fields.
type SessionInfo struct {
	// Data is provider-specific information. The schema is determined by the
	// provider.
	Data map[string]interface{}

	// TTL is the session duration.
	TTL time.Duration
}
