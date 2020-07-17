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
// Codes can be configured by creating an Request.
package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
)

// GenerateCode creates a new OTP code.
func GenerateCode(length uint) (string, error) {
	limit := big.NewInt(0)
	limit.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	digits, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", err
	}

	// The zero pad format is variable length based on the length of the request code.
	format := fmt.Sprint("%0", length, "d")
	result := fmt.Sprintf(format, digits.Int64())

	return result, nil
}

// Request represents the parameters of a verification code request.
type Request struct {
	DB            *database.Database
	Length        uint
	ExpiresAt     time.Time
	TestType      string
	SymptomDate   *time.Time
	MaxSymptomAge time.Duration
	IssuingUser   *database.User
	IssuingApp    *database.AuthorizedApp
}

// Issue wiill generate a verification code and save it to the database, based
// on the paremters provited.
func (o *Request) Issue(ctx context.Context, retryCount uint) (string, error) {
	logger := logging.FromContext(ctx)
	var code string
	var err error
	var i uint
	for i = 0; i < retryCount; i++ {
		code, err = GenerateCode(o.Length)
		if err != nil {
			logger.Errorf("code generation error: %v", err)
			continue
		}
		verificationCode := database.VerificationCode{
			Code:        code,
			TestType:    strings.ToLower(o.TestType),
			SymptomDate: o.SymptomDate,
			ExpiresAt:   o.ExpiresAt,
			IssuingUser: o.IssuingUser,
			IssuingApp:  o.IssuingApp,
		}
		// If a verification code already exists, it will fail to save, and we retry.
		if err := o.DB.SaveVerificationCode(&verificationCode, o.MaxSymptomAge); err != nil {
			logger.Warnf("duplicate OTP found: %v", err)
			continue
		} else {
			break // successful save, nil error, break out.
		}
	}
	return code, err

}
