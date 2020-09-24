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

// Package firebase is common logic and handling around firebase.
package firebase

const (
	EmailNotFound    = "EMAIL_NOT_FOUND"
	InvalidOOBCode   = "INVALID_OOB_CODE"
	CredentialTooOld = "CREDENTIAL_TOO_OLD_LOGIN_AGAIN"
	TokenExpired     = "TOKEN_EXPIRED"
	InvalidToken     = "INVALID_ID_TOKEN"
)

// ErrorDetails is the structure firebase gives back.
type ErrorDetails struct {
	ErrorCode int    `json:"code"`
	Error     string `json:"message"`
}

func (err *ErrorDetails) ShouldReauthenticate() bool {
	switch err.Error {
	case CredentialTooOld, TokenExpired, InvalidToken:
		return true
	}
	return false
}
