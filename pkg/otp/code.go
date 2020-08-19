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

	"github.com/google/exposure-notifications-server/pkg/logging"
)

const (
	// all lowercase characters plus 0-9
	charset = "abcdefghijklmnopqrstuvwxyz0123456789"
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

// GenerateAlphanumericCode will generate an alpha numberic code.
// It uses the length to estimate how many bytes of randomness will
// base64 encode to that length string.
// For example 16 character string requires 12 bytes.
func GenerateAlphanumericCode(length uint) (string, error) {
	var result string
	for i := uint(0); i < length; i++ {
		ch, err := randomFromCharset()
		if err != nil {
			return "", err
		}
		result = result + ch
	}
	return result, nil
}

func randomFromCharset() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return "", err
	}
	return string(charset[n.Int64()]), nil
}

// Request represents the parameters of a verification code request.
type Request struct {
	DB             *database.Database
	RealmID        uint
	ShortLength    uint
	ShortExpiresAt time.Time
	LongLength     uint
	LongExpiresAt  time.Time
	TestType       string
	SymptomDate    *time.Time
	MaxSymptomAge  time.Duration
	IssuingUser    *database.User
	IssuingApp     *database.AuthorizedApp
}

// Issue will generate a verification code and save it to the database, based on
// the paremters provided. It returns the short code, long code, a UUID for
// accessing the code, and any errors.
func (o *Request) Issue(ctx context.Context, retryCount uint) (string, string, string, error) {
	logger := logging.FromContext(ctx)
	var verificationCode database.VerificationCode
	var err error
	var code, longCode string
	for i := uint(0); i < retryCount; i++ {
		code, err = GenerateCode(o.ShortLength)
		if err != nil {
			logger.Errorf("code generation error: %v", err)
			continue
		}
		longCode = code
		if o.LongLength > 0 {
			longCode, err = GenerateAlphanumericCode(o.LongLength)
			if err != nil {
				logger.Errorf("long code generation error: %v", err)
				continue
			}
		}
		verificationCode = database.VerificationCode{
			RealmID:       o.RealmID,
			Code:          code,
			LongCode:      longCode,
			TestType:      strings.ToLower(o.TestType),
			SymptomDate:   o.SymptomDate,
			ExpiresAt:     o.ShortExpiresAt,
			LongExpiresAt: o.LongExpiresAt,
			IssuingUser:   o.IssuingUser,
			IssuingApp:    o.IssuingApp,
		}
		// If a verification code already exists, it will fail to save, and we retry.
		if err := o.DB.SaveVerificationCode(&verificationCode, o.MaxSymptomAge); err != nil {
			logger.Warnf("duplicate OTP found: %v", err)
			continue
		} else {
			break // successful save, nil error, break out.
		}
	}
	if err != nil {
		return "", "", "", err
	}
	return code, longCode, verificationCode.UUID, nil
}
