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

package issueapi

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

const (
	// all lowercase characters plus 0-9
	charset = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func (il *Controller) issueCode(ctx context.Context, vCode *database.VerificationCode, realm *database.Realm) *IssueResult {
	logger := logging.FromContext(ctx).Named("issueapi.populateCode")

	// If we got this far, we're about to issue a code - take from the limiter
	// to ensure this is permitted.
	if realm.AbusePreventionEnabled {
		key, err := realm.QuotaKey(il.config.GetRateLimitConfig().HMACKey)
		if err != nil {
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GENERATE_HMAC"),
				HTTPCode:    http.StatusInternalServerError,
				errorReturn: api.Error(err),
			}
		}
		limit, _, reset, ok, err := il.limiter.Take(ctx, key)
		if err != nil {
			logger.Errorw("failed to take from limiter", "error", err)
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_TAKE_FROM_LIMITER"),
				HTTPCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to verify realm stats, please try again"),
			}
		}

		stats.Record(ctx, mRealmTokenUsed.M(1))

		if !ok {
			logger.Warnw("realm has exceeded daily quota",
				"realm", realm.ID,
				"limit", limit,
				"reset", reset)

			if il.config.GetEnforceRealmQuotas() {
				return &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError("QUOTA_EXCEEDED"),
					HTTPCode:    http.StatusTooManyRequests,
					errorReturn: api.Errorf("exceeded realm quota, please contact a realm administrator").WithCode(api.ErrQuotaExceeded),
				}
			}
		}
	}

	if err := il.Issue(ctx, vCode, realm, il.config.GetCollisionRetryCount()); err != nil {
		logger.Errorw("failed to issue code", "error", err)
		// GormV1 doesn't have a good way to match db errors
		if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
				HTTPCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", vCode.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
		return &IssueResult{
			ObsBlame:    observability.BlameServer,
			ObsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
			HTTPCode:    http.StatusInternalServerError,
			errorReturn: api.Errorf("failed to generate otp code, please try again"),
		}
	}

	return &IssueResult{
		verCode:   vCode,
		HTTPCode:  http.StatusOK,
		ObsBlame:  observability.BlameNone,
		ObsResult: observability.ResultOK(),
	}
}

// Issue will generate a verification code and save it to the database, based on
// the paremters provided. It returns the short code, long code, a UUID for
// accessing the code, and any errors.
func (il *Controller) Issue(ctx context.Context, vCode *database.VerificationCode, realm *database.Realm, retryCount uint) error {
	logger := logging.FromContext(ctx)
	var err error
	for i := uint(0); i < retryCount; i++ {
		code, err := generateCode(realm.CodeLength)
		if err != nil {
			logger.Errorf("code generation error: %v", err)
			continue
		}
		longCode := code
		if realm.LongCodeLength > 0 {
			longCode, err = generateAlphanumericCode(realm.LongCodeLength)
			if err != nil {
				logger.Errorf("long code generation error: %v", err)
				continue
			}
		}
		vCode.Code = code
		vCode.LongCode = longCode

		// If a verification code already exists, it will fail to save, and we retry.
		if err = il.db.SaveVerificationCode(vCode, il.config.GetAllowedSymptomAge()); err != nil {
			logger.Warnf("duplicate OTP found: %v", err)
			if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
				break // not retryable
			}
			continue
		} else {
			// These are stored encrypted, but here we need to tell the user about them.
			vCode.Code = code
			vCode.LongCode = longCode
			break // successful save, nil error, break out.
		}
	}
	if err != nil {
		return err
	}
	return nil
}

// generateCode creates a new OTP code.
func generateCode(length uint) (string, error) {
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

// generateAlphanumericCode will generate an alpha numberic code.
// It uses the length to estimate how many bytes of randomness will
// base64 encode to that length string.
// For example 16 character string requires 12 bytes.
func generateAlphanumericCode(length uint) (string, error) {
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
