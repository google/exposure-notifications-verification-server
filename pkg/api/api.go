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

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

const (
	// TestTypeConfirmed is the string that represents a confirmed COVID-19 test.
	TestTypeConfirmed = "confirmed"
	// TestTypeLikely is the string that represents a clinical diagnosis.
	TestTypeLikely = "likely"
	// TestTypeNegative is the string that represents a negative test.
	TestTypeNegative = "negative"
	// TestTypeUserReport is the string that represents a user initiated report.
	TestTypeUserReport = "user-report"

	// error_code definitions for the APIs.

	// General

	// ErrUnparsableRequest indicates that the request could not be correctly parsed.
	ErrUnparsableRequest = "unparsable_request"
	// ErrInternal indicates some server-side error whose details are opaque to the caller.
	// this could mean a database or RPC connection drop or some other internal outage.
	ErrInternal = "internal_server_error"

	// Verify & Issue API responses

	// ErrVerifyCodeInvalid indicates the code entered is unknown or already used.
	ErrVerifyCodeInvalid = "code_invalid"
	// ErrVerifyCodeExpired indicates the code provided is known to the server, but expired.
	ErrVerifyCodeExpired = "code_expired"
	// ErrVerifyCodeNotFound indicates the code does not exist on the server/realm.
	ErrVerifyCodeNotFound = "code_not_found"
	// ErrVerifyCodeUserUnauth indicates the code does not belong to the requesting user.
	ErrVerifyCodeUserUnauth = "code_user_unauthorized"
	// ErrUnsupportedTestType indicates the client is unable to process the appropriate test type
	// in this case, the user should be directed to upgrade their app / operating system.
	// Accompanied by an HTTP status of StatusPreconditionFailed (412).
	ErrUnsupportedTestType = "unsupported_test_type"
	// ErrInvalidTestType indicates the client says it supports a test type this server doesn't
	// know about.
	ErrInvalidTestType = "invalid_test_type"
	// ErrMissingDate indicates the realm requires a date, but none was supplied.
	ErrMissingDate = "missing_date"
	// ErrInvalidDate indicates the realm requires a date, but the supplied date
	// was older or newer than the allowed date range.
	ErrInvalidDate = "invalid_date"
	// ErrUUIDAlreadyExists indicates that the UUID has already been used for an issued code.
	ErrUUIDAlreadyExists = "uuid_already_exists"
	// ErrMaintenanceMode indicates that the server is read-only for maintenance.
	ErrMaintenanceMode = "maintenance_mode"
	// ErrQuotaExceeded indicates the realm has exceeded its daily allotment of codes.
	ErrQuotaExceeded = "quota_exceeded"
	// ErrSMSQueueFull indicates that Twilio's SMS queue is full and may not accept more SMS messages to send.
	ErrSMSQueueFull = "sms_queue_full"
	// ErrSMSFailure indicates that Twilio's responded with a failure.
	ErrSMSFailure = "sms_failure"
	// ErrMissingNonce indicates a UserReport request is missing the nonce value.
	ErrMissingNonce = "missing_nonce"
	// ErrMissingPhone indicates a UserReport request is missing the phone number.
	ErrMissingPhone = "missing_phone"

	// User report specific responses
	// ErrUserReportTryLater indicates that user report is not allowed right now, which could be for several
	// reasons: phone number already used, PHA hit self report quota, PHA disabled self report.
	ErrUserReportTryLater = "user_report_try_later"

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

	// ErrorCodeLegacy exists to populate the JSON with a deprecated error_code
	// key. This will be removed in a future version. Consumers should use
	// `errorCode` instead.
	ErrorCodeLegacy string `json:"error_code"`
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
	e.ErrorCodeLegacy = code
	return e
}

// Padding is an optional field to change the size of the request or response.
// It's arbitrary bytes that should be ignored or discarded. It primarily exists
// to prevent a network observer from building a model based on request or
// response sizes.
type Padding []byte

// MarshalJSON is a custom JSON marshaler for padding. It generates and returns
// 1-2kb (random) of base64-encoded bytes.
func (p Padding) MarshalJSON() ([]byte, error) {
	if p == nil {
		bi, err := rand.Int(rand.Reader, big.NewInt(1024))
		if err != nil {
			return nil, fmt.Errorf("padding: failed to generate random number: %w", err)
		}

		// rand.Int is [0, max), so add 1kb to set the range from 1-2kb.
		i := int(bi.Int64() + 1024)

		p = make([]byte, i)
		n, err := rand.Read(p)
		if err != nil {
			return nil, fmt.Errorf("padding: failed to read bytes: %w", err)
		}
		if n < i {
			return nil, fmt.Errorf("padding: wrote less bytes than expected")
		}
	}

	s := fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(p))
	return []byte(s), nil
}

// UnmarshalJSON is a custom JSON unmarshaler for padding.
// The field is meaningless bytes, so this is just a passthrough.
func (p *Padding) UnmarshalJSON(b []byte) error {
	*p = bytes.Trim(b, `"`)
	return nil
}

// UserBatchRequest is a request for bulk creation of users.
// This is called by the Web frontend.
// API is served at /users/import/userbatch
type UserBatchRequest struct {
	Users       []BatchUser `json:"users"`
	SendInvites bool        `json:"sendInvites"`
}

// BatchUser represents a single user's email/name.
type BatchUser struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// UserBatchResponse defines the response type for UserBatchRequest.
type UserBatchResponse struct {
	NewUsers []*BatchUser `json:"newUsers"`

	Error     string `json:"error"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// IssueCodeRequest defines the parameters to request an new OTP (short term)
// code. This is called by the Web frontend.
// API is served at /api/issue
type IssueCodeRequest struct {
	Padding Padding `json:"padding"`

	SymptomDate string `json:"symptomDate"` // ISO 8601 formatted date, YYYY-MM-DD
	TestDate    string `json:"testDate"`
	TestType    string `json:"testType"`
	// Offset in minutes of the user's timezone. Positive, negative, 0, or omitted
	// (using the default of 0) are all valid. 0 is considered to be UTC.
	TZOffset         float32 `json:"tzOffset"`
	Phone            string  `json:"phone"`
	SMSTemplateLabel string  `json:"smsTemplateLabel"`

	// Optional: UUID is a handle which allows the issuer to track status
	// of the issued verification code. If omitted the server will generate the UUID.
	UUID string `json:"uuid"`

	// ExternalIssuerID is optional information supplied by the API caller to
	// uniquely identify the entity making this request. This is useful where
	// callers are using a single API key behind an ERP, or when callers are using
	// the verification server as an API with a different authentication system.
	// This field is optional.

	// The information provided is stored exactly as-is. If the identifier is
	// uniquely identifying PII (such as an email address, employee ID, SSN, etc),
	// the caller should apply a cryptographic hash before sending that data. The
	// system does not sanitize or encrypt these external IDs, it is the caller's
	// responsibility to do so.
	ExternalIssuerID string `json:"externalIssuerID"`
}

// IssueCodeResponse defines the response type for IssueCodeRequest.
type IssueCodeResponse struct {
	Padding Padding `json:"padding"`

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

	// LongExpiresAt and LongExpiresAtTimestamp represents the time when the long
	// code expires, in UTC seconds since epoch.
	LongExpiresAt          string `json:"longExpiresAt,omitempty"`
	LongExpiresAtTimestamp int64  `json:"longExpiresAtTimestamp,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// BatchIssueCodeRequest defines the request for issuing many codes at once.
type BatchIssueCodeRequest struct {
	Padding Padding             `json:"padding"`
	Codes   []*IssueCodeRequest `json:"codes"`
}

// BatchIssueCodeResponse defines the response for BatchIssueCodeRequest.
type BatchIssueCodeResponse struct {
	Padding Padding              `json:"padding"`
	Codes   []*IssueCodeResponse `json:"codes,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// CheckCodeStatusRequest defines the parameters to request the status for a
// previously issued OTP code. This is called by the Web frontend.
// API is served at /api/checkcodestatus
type CheckCodeStatusRequest struct {
	Padding Padding `json:"padding"`

	// UUID is a handle which allows the issuer to track status of the issued verification code.
	UUID string `json:"uuid"`
}

// CheckCodeStatusResponse defines the response type for CheckCodeStatusRequest.
type CheckCodeStatusResponse struct {
	Padding Padding `json:"padding"`

	// Claimed is true if a user has used the OTP code to get a token via the VerifyCode api.
	Claimed bool `json:"claimed"`

	// ExpiresAtTimestamp represents Unix, seconds since the epoch. Still UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAtTimestamp int64 `json:"expiresAtTimestamp"`

	// LongExpiresAtTimestamp repesents the time when the long code expires, in
	// UTC seconds since epoch.
	LongExpiresAtTimestamp int64 `json:"longExpiresAtTimestamp,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// ExpireCodeRequest defines the parameters to request that a code be expired now.
// This is called by the Web frontend.
// API is served at /api/expirecode
type ExpireCodeRequest struct {
	Padding Padding `json:"padding"`

	// UUID is a handle which allows the issuer to track status of the issued verification code.
	UUID string `json:"uuid"`
}

// ExpireCodeResponse defines the response type for ExpireCodeRequest.
type ExpireCodeResponse struct {
	Padding Padding `json:"padding"`

	// ExpiresAtTimestamp represents Unix, seconds since the epoch. Still UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAtTimestamp int64 `json:"expiresAtTimestamp"`

	// LongExpiresAtTimestamp represents the time when the long code expires, in
	// UTC seconds since epoch.
	LongExpiresAtTimestamp int64 `json:"longExpiresAtTimestamp,omitempty"`

	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
}

// UserReportRequest defines the structure for a user initiated report.
// This is a device API hosted on the apiserver.
//
// Support level: ALPHA
// This API is not guaranteed to be backwards compatible until such time it is moved
// to beta.
//
// Inclusion of the test date is required, inclusion of the symptom date is optional.
//
// A phone number is required. The verification code is only presented over SMS.
//
// The nonce field must be a 256 random bytes, base64 encoded.
// This nonce field must be passed back on the VerifyCodeRequest request later.
//
// Requires API key in a HTTP header, X-API-Key: APIKEY
type UserReportRequest struct {
	Padding Padding `json:"padding"`

	SymptomDate string `json:"symptomDate"` // ISO 8601 formatted date, YYYY-MM-DD
	TestDate    string `json:"testDate"`
	// TZOffset in minutes of the user's timezone. Positive, negative, 0, or omitted
	// (using the default of 0) are all valid. 0 is considered to be UTC.
	TZOffset float32 `json:"tzOffset"`
	// Phone is required in this API. The verification code will be delivered over SMS.
	Phone string `json:"phone"`

	// Nonce must be 256 bytes of random data, base64 encoded.
	Nonce string `json:"nonce"`
}

// UserReportResponse is the reply from a UserReportRequest.
//
type UserReportResponse struct {
	Padding Padding `json:"padding"`

	// ExpiresAt is a RFC1123 formatted string formatted timestamp, in UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAt string `json:"expiresAt"`

	// ExpiresAtTimestamp represents Unix, seconds since the epoch. Still UTC.
	// After this time the code will no longer be accepted and is eligible for deletion.
	ExpiresAtTimestamp int64 `json:"expiresAtTimestamp"`

	Error     string `json:"error,omitempty"`
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
// The special type of test, '"user-report"' can be added safely to any of the other types.
//
// The nonce must be provided when validating a self report key.
//
// Requires API key in a HTTP header, X-API-Key: APIKEY
type VerifyCodeRequest struct {
	Padding Padding `json:"padding"`

	VerificationCode string   `json:"code"`
	AcceptTestTypes  []string `json:"accept"`

	// Optional: If present must be the same nonce that was used to request the verification code.
	// base64 encoded.
	Nonce string `json:"nonce,omitempty"`
}

// VerifyCodeResponse either contains an error, or contains the test parameters
// (type and [optional] date) as well as the verification token. The verification token
// may be sent back on a valid VerificationCertificateRequest later.
type VerifyCodeResponse struct {
	Padding Padding `json:"padding"`

	TestType          string `json:"testtype,omitempty"`
	SymptomDate       string `json:"symptomDate,omitempty"` // ISO 8601 formatted date, YYYY-MM-DD
	TestDate          string `json:"testDate,omitempty"`    // ISO 8601 formatted date, YYYY-MM-DD
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
	Padding Padding `json:"padding"`

	VerificationToken string `json:"token"`
	ExposureKeyHMAC   string `json:"ekeyhmac"`
}

// VerificationCertificateResponse either contains an error or contains
// a signed certificate that can be presented to the configured exposure
// notifications server to publish keys along w/ the certified diagnosis.
type VerificationCertificateResponse struct {
	Padding Padding `json:"padding"`

	Certificate string `json:"certificate,omitempty"`
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
}
