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
	// TestTypeNegative is the string that represents a netgative test.
	TestTypeNegative = "negative"
)

// ErrorReturn defines the common error type.
type ErrorReturn struct {
	Error string `json:"error"`
}

// Error creates an ErrorReturn w/ the formateed message.
func Error(msg string, vars ...interface{}) *ErrorReturn {
	return &ErrorReturn{Error: fmt.Sprintf(msg, vars...)}
}

// CSRFResponse is the return type when requesting an AJAX CSRF token.
type CSRFResponse struct {
	CSRFToken string `json:"csrftoken"`
	Error     string `json:"error"`
}

// IssueCodeRequest defines the parameters to request an new OTP (short term)
// code. This is called by the Web frontend.
// API is served at /api/issue
type IssueCodeRequest struct {
	TestType    string `json:"testType"`
	SymptomDate string `json:"symptomDate"`
}

// IssueCodeResponse defines the response type for IssueCodeRequest.
type IssueCodeResponse struct {
	VerificationCode string `json:"code"`
	ExpiresAt        string `json:"expiresAt"`
	Error            string `json:"error"`
}

// VerifyCodeRequest is the request structure for exchanging a shor term Verification Code
// (OTP) for a long term token (a JWT) that can later be used to sign TEKs.
//
// Requires API key in a HTTP header, X-API-Key: APIKEY
type VerifyCodeRequest struct {
	VerificationCode string `json:"code"`
}

// VerifyCodeResponse either contains an error, or contains the test parameters
// (type and [optional] date) as well as the verification token. The verification token
// may be snet back on a valid VerificationCertificateRequest later.
type VerifyCodeResponse struct {
	TestType          string `json:"testtype"`
	SymptomDate       string `json:"symptomDate"` // ISO 8601 formatted date, YYYY-MM-DD
	VerificationToken string `json:"token"`       // JWT - signed, not encrypted.
	Error             string `json:"error"`
}

// VerificationCertificateRequest is used to accept a long term token and
// an HMAC of the TEKs.
// The details of the HMAC calculation are avialble at:
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
	Certificate string `json:"certificate"`
	Error       string `json:"error"`
}
