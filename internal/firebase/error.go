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

import "errors"

var (
	EmailNotFound    = &ErrorDetails{Err: "EMAIL_NOT_FOUND"}
	InvalidOOBCode   = &ErrorDetails{Err: "INVALID_OOB_CODE"}
	CredentialTooOld = &ErrorDetails{Err: "CREDENTIAL_TOO_OLD_LOGIN_AGAIN"}
	TokenExpired     = &ErrorDetails{Err: "TOKEN_EXPIRED"}
	InvalidToken     = &ErrorDetails{Err: "INVALID_ID_TOKEN"}
)

// ErrorDetails is the structure firebase gives back.
type ErrorDetails struct {
	ErrorCode int    `json:"code"`
	Err       string `json:"message"`
	Message   string
}

func (err *ErrorDetails) Error() string {
	return err.Message
}

func (err *ErrorDetails) Is(target error) bool {
	if t, ok := target.(*ErrorDetails); ok {
		return err.Err == t.Err
	}
	return err.Message == target.Error()
}

// ShouldReauthenticate returns true for errors that require a refreshed auth token.
func (err *ErrorDetails) ShouldReauthenticate() bool {
	return errors.Is(err, CredentialTooOld) ||
		errors.Is(err, TokenExpired) ||
		errors.Is(err, InvalidToken)
}
