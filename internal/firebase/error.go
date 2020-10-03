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

package firebase

import (
	"errors"
)

var (
	ErrEmailNotFound    = &ErrorDetails{Err: "EMAIL_NOT_FOUND"}
	ErrInvalidOOBCode   = &ErrorDetails{Err: "INVALID_OOB_CODE"}
	ErrExpiredOOBCode   = &ErrorDetails{Err: "EXPIRED_OOB_CODE"}
	ErrCredentialTooOld = &ErrorDetails{Err: "CREDENTIAL_TOO_OLD_LOGIN_AGAIN"}
	ErrTokenExpired     = &ErrorDetails{Err: "TOKEN_EXPIRED"}
	ErrInvalidToken     = &ErrorDetails{Err: "INVALID_ID_TOKEN"}
	ErrTooManyAttempts  = &ErrorDetails{Err: "TOO_MANY_ATTEMPTS_TRY_LATER"}
)

var _ error = (*ErrorDetails)(nil)

// ErrorDetails is the structure firebase gives back.
type ErrorDetails struct {
	ErrorCode int    `json:"code"`
	Err       string `json:"message"`
}

func (err *ErrorDetails) Error() string {
	return err.Err
}

func (err *ErrorDetails) Is(target error) bool {
	if tErr, ok := target.(*ErrorDetails); ok {
		return err.Err == tErr.Err
	}
	return false
}

// ShouldReauthenticate returns true for errors that require a refreshed auth token.
func (err *ErrorDetails) ShouldReauthenticate() bool {
	return errors.Is(err, ErrCredentialTooOld) ||
		errors.Is(err, ErrTokenExpired) ||
		errors.Is(err, ErrInvalidToken)
}
