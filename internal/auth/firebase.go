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
	"fmt"

	firebaseinternal "github.com/google/exposure-notifications-verification-server/internal/firebase"
	"github.com/sethvargo/go-password/password"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/gorilla/sessions"
)

const (
	sessionKeyFirebaseCookie = sessionKey("firebaseCookie")
)

type firebaseAuth struct {
	firebaseAuth     *auth.Client
	firebaseInternal *firebaseinternal.Client
}

// NewFirebase creates a new auth provider for firebase.
func NewFirebase(ctx context.Context, config *firebase.Config) (Provider, error) {
	app, err := firebase.NewApp(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create firebase app: %w", err)
	}

	auth, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to configure firebase auth: %w", err)
	}

	internal, err := firebaseinternal.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to configure firebase client: %w", err)
	}

	return &firebaseAuth{
		firebaseAuth:     auth,
		firebaseInternal: internal,
	}, nil
}

// CheckRevoked checks if the users auth has been revoked.
func (f *firebaseAuth) CheckRevoked(ctx context.Context, session *sessions.Session) error {
	raw, err := sessionGet(session, sessionKeyFirebaseCookie)
	if err != nil {
		f.ClearSession(ctx, session)
		return err
	}

	cookie, ok := raw.(string)
	if !ok || cookie == "" {
		f.ClearSession(ctx, session)
		return ErrSessionMissing
	}

	if _, err := f.firebaseAuth.VerifySessionCookieAndCheckRevoked(ctx, cookie); err != nil {
		f.ClearSession(ctx, session)
		return err
	}
	return nil
}

// StoreSession stores information about the session.
func (f *firebaseAuth) StoreSession(ctx context.Context, session *sessions.Session, i *SessionInfo) error {
	if i == nil || i.Data == nil {
		f.ClearSession(ctx, session)
		return ErrSessionInfoMissing
	}

	idToken, ok := i.Data["id_token"].(string)
	if !ok {
		f.ClearSession(ctx, session)
		return fmt.Errorf("missing id_token: %w", ErrSessionInfoMissing)
	}

	// Convert ID token to long-lived cookie
	cookie, err := f.firebaseAuth.SessionCookie(ctx, idToken, i.TTL)
	if err != nil {
		f.ClearSession(ctx, session)
		return err
	}

	// Set cookie
	if err := sessionSet(session, sessionKeyFirebaseCookie, cookie); err != nil {
		f.ClearSession(ctx, session)
		return err
	}

	return nil
}

// ClearSession removes any session information for this auth.
func (f *firebaseAuth) ClearSession(ctx context.Context, session *sessions.Session) {
	sessionClear(session, sessionKeyFirebaseCookie)
}

// CreateUser creates a user in the upstream auth system with the given name and
// email. It returns true if the user was created or false if the user already
// exists.
func (f *firebaseAuth) CreateUser(ctx context.Context, name, email, pass string, sendInvite bool, emailer InviteUserEmailFunc) (bool, error) {
	// Attempt to get the user by email. If that returns successfully, it means
	// the user exists.
	user, err := f.firebaseAuth.GetUserByEmail(ctx, email)
	if err != nil && !auth.IsUserNotFound(err) {
		return false, fmt.Errorf("failed lookup firebase user: %w", err)
	}

	if user == nil {
		// If the password is empty, generate one.
		if pass == "" {
			pass, err = password.Generate(24, 8, 8, false, true)
			if err != nil {
				return false, fmt.Errorf("failed to generate password: %w", err)
			}
		}

		// Create the user.
		userToCreate := (&auth.UserToCreate{}).
			Email(email).
			Password(pass).
			DisplayName(name)
		if _, err = f.firebaseAuth.CreateUser(ctx, userToCreate); err != nil {
			return false, fmt.Errorf("failed to create firebase user: %w", err)
		}
	}

	// Send the welcome email. Use the defined mailer if given, otherwise fallback
	// to firebase default.
	if sendInvite {
		if emailer != nil {
			inviteLink, err := f.passwordResetLink(ctx, email)
			if err != nil {
				return true, err
			}

			if err := emailer(ctx, inviteLink); err != nil {
				return true, fmt.Errorf("failed to send new user invitation email: %w", err)
			}
		} else {
			if err := f.firebaseInternal.SendNewUserInvitation(ctx, email); err != nil {
				return true, fmt.Errorf("failed to send new user invitation firebase email: %w", err)
			}
		}
	}

	return true, nil
}

// EmailAddress extracts the users email from the session.
func (f *firebaseAuth) EmailAddress(ctx context.Context, session *sessions.Session) (string, error) {
	data, err := f.loadCookie(ctx, session)
	if err != nil {
		return "", err
	}
	return data.Email, nil
}

// EmailVerified returns true if the current user is verified, false otherwise.
func (f *firebaseAuth) EmailVerified(ctx context.Context, session *sessions.Session) (bool, error) {
	data, err := f.loadCookie(ctx, session)
	if err != nil {
		return false, err
	}
	return data.EmailVerified, nil
}

// MFAEnabled returns whether MFA is enabled on the account.
func (f *firebaseAuth) MFAEnabled(ctx context.Context, session *sessions.Session) (bool, error) {
	data, err := f.loadCookie(ctx, session)
	if err != nil {
		return false, err
	}
	return data.MFAEnabled, nil
}

// ChangePassword changes the users password. The data must be an oobCode as a
// string.
func (f *firebaseAuth) ChangePassword(ctx context.Context, newPassword string, data interface{}) error {
	code, ok := data.(string)
	if !ok {
		return fmt.Errorf("missing or invalid oobCode")
	}

	if _, err := f.firebaseInternal.ChangePasswordWithCode(ctx, code, newPassword); err != nil {
		return err
	}
	return nil
}

// SendResetPasswordEmail resets the password for the given user. If the user does not
// exist, an error is returned.
func (f *firebaseAuth) SendResetPasswordEmail(ctx context.Context, email string, emailer ResetPasswordEmailFunc) error {
	// Send the reset email. Use SMTP emailer if defined, otherwise fallback to
	// the firebase default mailer.
	if emailer != nil {
		resetLink, err := f.passwordResetLink(ctx, email)
		if err != nil {
			return err
		}

		if err := emailer(ctx, resetLink); err != nil {
			return fmt.Errorf("failed to send password reset email: %w", err)
		}
	} else {
		if err := f.firebaseInternal.SendNewUserInvitation(ctx, email); err != nil {
			return fmt.Errorf("failed to send password reset firebase email: %w", err)
		}
	}

	return nil
}

// VerifyPasswordResetCode verifies the password reset code and returns the
// email address for the user.
func (f *firebaseAuth) VerifyPasswordResetCode(ctx context.Context, code string) (string, error) {
	email, err := f.firebaseInternal.VerifyPasswordResetCode(ctx, code)
	if err != nil {
		return "", err
	}
	return email, nil
}

// SendEmailVerificationEmail sends an message to the currently authenticated
// user, asking them to verify ownership of the email address.
func (f *firebaseAuth) SendEmailVerificationEmail(ctx context.Context, email string, data interface{}, emailer EmailVerificationEmailFunc) error {
	// Send the reset email. Use SMTP emailer if defined, otherwise fallback to
	// the firebase default mailer.
	if emailer != nil {
		verifyLink, err := f.emailVerificationLink(ctx, email)
		if err != nil {
			return err
		}

		if err := emailer(ctx, verifyLink); err != nil {
			return fmt.Errorf("failed to send email verification email: %w", err)
		}
	} else {
		if data == nil {
			return fmt.Errorf("firebase requires a fresh ID token to send email verifications")
		}
		idToken, ok := data.(string)
		if !ok {
			return fmt.Errorf("id token must be a string")
		}

		if err := f.firebaseInternal.SendEmailVerification(ctx, idToken); err != nil {
			return fmt.Errorf("failed to send email verification firebase email: %w", err)
		}
	}

	return nil
}

// passwordResetLink generates and returns the password reset link for the given
// email (user).
func (f *firebaseAuth) passwordResetLink(ctx context.Context, email string) (string, error) {
	reset, err := f.firebaseAuth.PasswordResetLink(ctx, email)
	if err != nil {
		return "", fmt.Errorf("failed to generate password reset link: %w", err)
	}
	return reset, nil
}

// emailVerificationLink generates an email verification link for the given
// email.
func (f *firebaseAuth) emailVerificationLink(ctx context.Context, email string) (string, error) {
	verify, err := f.firebaseAuth.EmailVerificationLink(ctx, email)
	if err != nil {
		return "", fmt.Errorf("failed to generate email verification link: %w", err)
	}
	return verify, nil
}

type firebaseCookieData struct {
	Email         string
	EmailVerified bool
	MFAEnabled    bool
}

// dataFromCookie extracts the information from the provided firebase cookie, if
// it exists.
func (f *firebaseAuth) dataFromCookie(ctx context.Context, cookie string) (*firebaseCookieData, error) {
	token, err := f.firebaseAuth.VerifySessionCookie(ctx, cookie)
	if err != nil {
		return nil, fmt.Errorf("failed to verify firebase cookie: %w", err)
	}

	if token.Claims == nil {
		return nil, fmt.Errorf("token claims are empty")
	}

	// Email
	email, ok := token.Claims["email"].(string)
	if !ok {
		return nil, fmt.Errorf("token claims for email are not a string")
	}

	// Email verified
	emailVerified, ok := token.Claims["email_verified"].(bool)
	if !ok {
		return nil, fmt.Errorf("token claims for email_verified are not a bool")
	}

	// MFA
	firebase, ok := token.Claims["firebase"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("token claims for firebase are missing")
	}
	_, mfaEnabled := firebase["sign_in_second_factor"]

	return &firebaseCookieData{
		Email:         email,
		EmailVerified: emailVerified,
		MFAEnabled:    mfaEnabled,
	}, nil
}

// loadCookie loads and parses the firebase cookie from the session.
func (f *firebaseAuth) loadCookie(ctx context.Context, session *sessions.Session) (*firebaseCookieData, error) {
	raw, err := sessionGet(session, sessionKeyFirebaseCookie)
	if err != nil {
		f.ClearSession(ctx, session)
		return nil, err
	}

	cookie, ok := raw.(string)
	if !ok || cookie == "" {
		f.ClearSession(ctx, session)
		return nil, ErrSessionMissing
	}

	data, err := f.dataFromCookie(ctx, cookie)
	if err != nil {
		f.ClearSession(ctx, session)
		return nil, err
	}
	return data, nil
}
