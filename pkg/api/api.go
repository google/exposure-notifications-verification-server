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

// Package api defines the JSON-RPC API between the browser and the server as
// well as between mobile devices and the server.
package api

import "fmt"

const (
	// TestTypeConfirmed is the string that represents a confirmed covid-19 test.
	TestTypeConfirmed = "confirmed"
	// TestTypeLikely is the string that represents a clinical diagnosis.
	TestTypeLikely = "likely"
	// TestTypeNegative is the string that represents a negative test.
	TestTypeNegative = "negative"

	// error_code definitions for the APIs.
	// General
	ErrUnparsableRequest = "unparsable_request"
	ErrInternal          = "internal_server_error"

	// Verify API responses
	// ErrVerifyCodeInvalid indicates the code entered is unknown or already used.
	ErrVerifyCodeInvalid = "code_invalid"
	// ErrVerifyCodeExpired indicates the code provided is known to the server, but expired.
	ErrVerifyCodeExpired = "code_expired"
	// ErrVerifyCodeNotFound indicates the code does not exist on the server/realm.
	ErrVerifyCodeNotFound = "code_not_found"
	// ErrVerifyCodeUserUnauth indicates the code does not belong to the requesting user.
	ErrVerifyCodeUserUnauth = "code_user_unauthorized"
	// ErrUnsupportedTestType indicates the client is unable to process the appropriate test type
	// in thise case, the user should be directed to upgrade their app / operating system.
	// Accompanied by an HTTP status of StatusPreconditionFailed (412).
	ErrUnsupportedTestType = "unsupported_test_type"
	// ErrInvalidTestType indicates the client says it supports a test type this server doesn't
	// know about.
	ErrInvalidTestType = "invalid_test_type"

	// Certificate API responses
	// ErrTokenInvalid indicates the token provided is unknown or already used
	ErrTokenInvalid = "token_invalid"
	// ErrTokenExpired indicates that the token provided is known but expired.
	ErrTokenExpired = "token_expired"
	// ErrHMACInvalid indicates that the HMAC that is being signed is invalid (wrong length)
	ErrHMACInvalid = "hmac_invalid"
)

// ErrorReturn defines the common error type.
type ErrorReturn struct {
	Error     string `json:"error"`
	ErrorCode string `json:"errorCode"`
}

// InternalError constructs a generic internal error.
func InternalError() *ErrorReturn {
	return Errorf("internal error").WithCode(ErrInternal)
}

// Errorf creates an ErrorReturn w/ the formateed message.
func Errorf(msg string, vars ...interface{}) *ErrorReturn {
	return &ErrorReturn{Error: fmt.Sprintf(msg, vars...)}
}

// Error wraps the error into an API error.
func Error(err error) *ErrorReturn {
	if err == nil {
		return nil
	}

	return &ErrorReturn{Error: err.Error()}
}

// WithCode adds an error code to an ErrorReturn
func (e *ErrorReturn) WithCode(code string) *ErrorReturn {
	e.ErrorCode = code
	return e
}

// CSRFResponse is the return type when requesting an AJAX CSRF token.
type CSRFResponse struct {
	CSRFToken string `json:"csrftoken"`
	Error     string `json:"error"`
	ErrorCode string `json:"errorCode"`
}

// IssueCodeRequest defines the parameters to request an new OTP (short term)
// code. This is called by the Web frontend.
// API is served at /api/issue
type IssueCodeRequest struct {
	SymptomDate string `json:"symptomDate"` // ISO 8601 formatted date, YYYY-MM-DD
	TestDate    string `json:"testDate"`
	TestType    string `json:"testType"`
	Phone       string `json:"phone"`
}

// IssueCodeResponse defines the response type for IssueCodeRequest.
type IssueCodeResponse struct {
	// UUID is a handle which allows the issuer to track status of the issued verification code.
	UUID string `json:"uuid"`

	// The OTP code which may be exchanged by the user for a signing token.
	VerificationCode string `json:"code"`

	// ExpiresAt is a RFC1123 formatted string formatted timestamp, in UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAt string `json:"expiresAt"`

	// ExpiresAtTimestamp represents Unix, seconds since the epoch. Still UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAtTimestamp int64 `json:"expiresAtTimestamp"`

	Error     string `json:"error"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// CheckCodeStatusRequest defines the parameters to request the status for a
// previously issued OTP code. This is called by the Web frontend.
// API is served at /api/checkcodestatus
type CheckCodeStatusRequest struct {
	// UUID is a handle which allows the issuer to track status of the issued verification code.
	UUID string `json:"uuid"`
}

// CheckCodeStatusResponse defines the response type for CheckCodeStatusRequest.
type CheckCodeStatusResponse struct {
	// Claimed is true if a user has used the OTP code to get a token via the VerifyCode api.
	Claimed bool `json:"claimed"`

	// ExpiresAtTimestamp represents Unix, seconds since the epoch. Still UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAtTimestamp int64 `json:"expiresAtTimestamp"`

	Error     string `json:"error"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// VerifyCodeRequest is the request structure for exchanging a short term Verification Code
// (OTP) for a long term token (a JWT) that can later be used to sign TEKs.
//
// 'code' is either the issued short code or long code issued to the user. Either one is
//   acceptable. Note that they normally have different expiry times.
// 'accept' is a list of accepted test types by the client. Acceptable values are
//   - ["confirmed"]
//   - ["confirmed", "likely"]  == ["likely"]
//   - ["confirmed", "likely", "negative"] == ["negative"]
//   These values form a hierarchy, if a client will accept 'likely' they must accept
//   both confirmed and likely. 'negative' indicates you accept confirmed, likely, and negative.
//   A client can pass in the complete list they accept or the "highest" value they can accept.
//   If this value is omitted or is empty, the client agrees to accept ALL possible
//   test types, including test types that may be introduced in the future.
//
//
// Requires API key in a HTTP header, X-API-Key: APIKEY
type VerifyCodeRequest struct {
	VerificationCode string   `json:"code"`
	AcceptTestTypes  []string `json:"accept,omitempty"`
}

// VerifyCodeResponse either contains an error, or contains the test parameters
// (type and [optional] date) as well as the verification token. The verification token
// may be sent back on a valid VerificationCertificateRequest later.
type VerifyCodeResponse struct {
	TestType          string `json:"testtype,omitempty"`
	SymptomDate       string `json:"symptomDate,omitempty"` // ISO 8601 formatted date, YYYY-MM-DD
	VerificationToken string `json:"token,omitempty"`       // JWT - signed, not encrypted.
	Error             string `json:"error,omitempty"`
	ErrorCode         string `json:"errorCode,omitempty"`
}

// VerificationCertificateRequest is used to accept a long term token and
// an HMAC of the TEKs.
// The details of the HMAC calculation are available at:
// https://github.com/google/exposure-notifications-server/blob/main/docs/design/verification_protocol.md
//
// Requires API key in a HTTP header, X-API-Key: APIKEY
type VerificationCertificateRequest struct {
	VerificationToken string `json:"token"`
	ExposureKeyHMAC   string `json:"ekeyhmac"`
}

// VerificationCertificateResponse either contains an error or contains
// a signed certificate that can be presented to the configured exposure
// notifications server to publish keys along w/ the certified diagnosis.
type VerificationCertificateResponse struct {
	Certificate string `json:"certificate,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
}
