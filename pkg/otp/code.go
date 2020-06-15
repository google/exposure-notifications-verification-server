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

// Package otp contains the implementation of the issuance of verification codes.
// Codes can be configured by creating an OTPRequest.
package otp

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
)

const (
	charSet   = "0123456789"
	setLength = 10
)

// GenerateCode creates a new OTP code.
func generateCode(length int) (string, error) {
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	// Reassign the value in the buffer to the corresponding charSet entry.
	for i := 0; i < length; i++ {
		buffer[i] = charSet[int(buffer[i])%setLength]
	}

	return string(buffer), nil
}

// OTPRequest represents the parameters of a verification code request.
type OTPRequest struct {
	DB         *database.Database
	Length     int
	ExpiresAt  time.Time
	TestType   string
	TestDate   *time.Time
	MaxTestAge time.Duration
}

// Issue wiill generate a verification code and save it to the database, based
// on the paremters provited.
func (o *OTPRequest) Issue(ctx context.Context, retryCount int) (string, error) {
	logger := logging.FromContext(ctx)
	var code string
	var err error
	for i := 0; i < retryCount; i++ {
		code, err = generateCode(o.Length)
		if err != nil {
			logger.Errorf("code generation error: %v", err)
			continue
		}
		verificationCode := database.VerificationCode{
			Code:      code,
			TestType:  strings.ToLower(o.TestType),
			TestDate:  o.TestDate,
			ExpiresAt: o.ExpiresAt,
		}
		// If a verification code already exists, it will fail to save, and we retry.
		if err := o.DB.SaveVerificationCode(&verificationCode, o.MaxTestAge); err != nil {
			logger.Warnf("duplicate OTP found: %v", err)
			continue
		} else {
			break // successful save, nil error, break out.
		}
	}
	return code, err

}
