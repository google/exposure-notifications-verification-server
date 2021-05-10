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
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/sethvargo/go-retry"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

const (
	// all lowercase characters plus 0-9
	charset = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func (c *Controller) IssueCode(ctx context.Context, vCode *database.VerificationCode, realm *database.Realm) *IssueResult {
	logger := logging.FromContext(ctx).Named("issueapi.IssueCode")

	// If we got this far, we're about to issue a code - take from the limiter
	// to ensure this is permitted.
	if realm.AbusePreventionEnabled {
		key, err := realm.QuotaKey(c.config.GetRateLimitConfig().HMACKey)
		if err != nil {
			return &IssueResult{
				obsResult:   enobs.ResultError("FAILED_TO_GENERATE_HMAC"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Error(err).WithCode(api.ErrInternal),
			}
		}

		if limit, _, reset, ok, err := c.limiter.Take(ctx, key); err != nil {
			logger.Errorw("failed to take from limiter", "error", err)
			return &IssueResult{
				obsResult:   enobs.ResultError("FAILED_TO_TAKE_FROM_LIMITER"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Errorf("failed to issue code, please try again in a few seconds").WithCode(api.ErrInternal),
			}
		} else if !ok {
			logger.Warnw("realm has exceeded daily quota",
				"realm", realm.ID,
				"limit", limit,
				"reset", reset)

			if c.config.IssueConfig().EnforceRealmQuotas {
				return &IssueResult{
					obsResult:   enobs.ResultError("QUOTA_EXCEEDED"),
					HTTPCode:    http.StatusTooManyRequests,
					ErrorReturn: api.Errorf("exceeded daily realm quota configured from abuse prevention, please contact a realm administrator").WithCode(api.ErrQuotaExceeded),
				}
			}
		}
		stats.Record(ctx, mRealmTokenUsed.M(1))
	}

	if err := c.CommitCode(ctx, vCode, realm, c.config.IssueConfig().CollisionRetryCount); err != nil {
		if errors.Is(err, database.ErrAlreadyReported) {
			stats.Record(ctx, mUserReportColission.M(1))
			return &IssueResult{
				VerCode:     vCode,
				obsResult:   enobs.ResultError("DUPLICATE_USER_REPORT"),
				HTTPCode:    http.StatusConflict,
				ErrorReturn: api.Errorf("phone number not currently eligible for user report").WithCode(api.ErrUserReportTryLater),
			}
		}
		if errors.Is(err, database.ErrRequiresPhoneNumber) {
			return &IssueResult{
				obsResult:   enobs.ResultError("MISSING_PHONE_NUMBER"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Errorf("phone number is required when initiating a user report").WithCode(api.ErrMissingPhone),
			}
		}
		logger.Errorw("failed to issue code", "error", err)
		return &IssueResult{
			obsResult:   enobs.ResultError("FAILED_TO_ISSUE_CODE"),
			HTTPCode:    http.StatusInternalServerError,
			ErrorReturn: api.Errorf("failed to generate otp code, please try again").WithCode(api.ErrInternal),
		}
	}

	return &IssueResult{
		VerCode:   vCode,
		HTTPCode:  http.StatusOK,
		obsResult: enobs.ResultOK,
	}
}

// CommitCode will generate a verification code and save it to the database, based on
// the paremters provided. It returns the short code, long code, a UUID for
// accessing the code, and any errors.
func (c *Controller) CommitCode(ctx context.Context, vCode *database.VerificationCode, realm *database.Realm, retryCount uint) error {
	b, err := retry.NewConstant(50 * time.Millisecond)
	if err != nil {
		return err
	}

	if err := retry.Do(ctx, retry.WithMaxRetries(uint64(retryCount), b), func(ctx context.Context) error {
		code, err := GenerateCode(realm.CodeLength)
		if err != nil {
			return err
		}
		longCode := code
		if realm.LongCodeLength > 0 {
			longCode, err = GenerateAlphanumericCode(realm.LongCodeLength)
			if err != nil {
				return err
			}
		}
		vCode.Code = code
		vCode.LongCode = longCode

		// If a verification code already exists, it will fail to save, and we retry.
		err = c.db.SaveVerificationCode(vCode, realm)
		switch {
		case err == nil:
			// These are stored encrypted, but here we need to tell the user about them.
			vCode.Code = code
			vCode.LongCode = longCode
			return nil // success
		case strings.Contains(err.Error(), database.VerCodesCodeUniqueIndex),
			strings.Contains(err.Error(), database.VerCodesLongCodeUniqueIndex):
			return retry.RetryableError(err)
		default:
			return err // err not retryable
		}
	}); err != nil {
		return err
	}

	return nil
}

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
