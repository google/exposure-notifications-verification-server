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
	EmailNotFound    = errors.New("EMAIL_NOT_FOUND")
	InvalidOOBCode   = errors.New("INVALID_OOB_CODE")
	CredentialTooOld = errors.New("CREDENTIAL_TOO_OLD_LOGIN_AGAIN")
	TokenExpired     = errors.New("TOKEN_EXPIRED")
	InvalidToken     = errors.New("INVALID_ID_TOKEN")
)

// ErrorDetails is the structure firebase gives back.
type ErrorDetails struct {
	ErrorCode int    `json:"code"`
	Err       string `json:"message"`
}

func (err ErrorDetails) Error() string {
	return err.Err
}

// ShouldReauthenticate returns true for errors that require a refreshed auth token.
func (err ErrorDetails) ShouldReauthenticate() bool {
	return errors.Is(err, CredentialTooOld) ||
		errors.Is(err, TokenExpired) ||
		errors.Is(err, InvalidToken)
}
