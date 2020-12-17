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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"go.opencensus.io/stats"
)

func (c *Controller) issueMany(ctx context.Context, authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm,
	requests []*api.IssueCodeRequest) []*api.IssueCodeResponse {

	// Generate codes
	results := make([]*issueResult, len(requests))
	for i, singleReq := range requests {
		results[i] = c.generateCode(ctx, authApp, membership, realm, singleReq)
	}

	// Send SMS messages
	for i, result := range results {
		c.sendSMS(ctx, realm, requests[i], result)
	}

	// Convert to API response
	responses := make([]*api.IssueCodeResponse, len(requests))
	for i, result := range results {
		v := result.verCode
		responses[i] = &api.IssueCodeResponse{
			UUID:                   v.UUID,
			VerificationCode:       v.Code,
			ExpiresAt:              v.ExpiresAt.Format(time.RFC1123),
			ExpiresAtTimestamp:     v.ExpiresAt.UTC().Unix(),
			LongExpiresAt:          v.LongExpiresAt.Format(time.RFC1123),
			LongExpiresAtTimestamp: v.LongExpiresAt.UTC().Unix(),
		}
	}
	return responses
}

// generateCode issues a code. Footgun: Does not send SMS
func (c *Controller) generateCode(ctx context.Context, authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm,
	request *api.IssueCodeRequest) *issueResult {
	logger := logging.FromContext(ctx).Named("issueapi.generateCode")

	// If this realm requires a date but no date was specified, return an error.
	if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !realm.ValidTestType(request.TestType) {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify the test type
	request.TestType = strings.ToLower(request.TestType)
	if _, ok := c.validTestType[request.TestType]; !ok {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("invalid test type").WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err := realm.SMSProvider(c.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to get sms provider"),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusBadRequest,
				errorReturn: api.Error(err),
			}
		}
	}

	// Set up parallel arrays to leverage the observability reporting and connect the parse / validation errors
	// to the correct date.
	parsedDates := make([]*time.Time, 2)
	input := []string{request.SymptomDate, request.TestDate}
	dateSettings := []*dateParseSettings{&onsetSettings, &testSettings}
	for i, d := range input {
		if d != "" {
			parsed, err := time.Parse(project.RFC3339Date, d)
			if err != nil {
				return &issueResult{
					obsBlame:    observability.BlameClient,
					obsResult:   observability.ResultError(dateSettings[i].ParseError),
					httpCode:    http.StatusBadRequest,
					errorReturn: api.Errorf("failed to process %s date: %v", dateSettings[i].Name, err).WithCode(api.ErrUnparsableRequest),
				}
			}
			// Max date is today (UTC time) and min date is AllowedTestAge ago, truncated.
			maxDate := timeutils.UTCMidnight(time.Now())
			minDate := timeutils.Midnight(maxDate.Add(-1 * c.config.GetAllowedSymptomAge()))

			validatedDate, err := validateDate(parsed, minDate, maxDate, int(request.TZOffset))
			if err != nil {
				err := fmt.Errorf("%s date must be on/after %v and on/before %v %v",
					dateSettings[i].Name,
					minDate.Format(project.RFC3339Date),
					maxDate.Format(project.RFC3339Date),
					parsed.Format(project.RFC3339Date),
				)
				return &issueResult{
					obsBlame:    observability.BlameClient,
					obsResult:   observability.ResultError(dateSettings[i].ValidateError),
					httpCode:    http.StatusBadRequest,
					errorReturn: api.Error(err),
				}
			}
			parsedDates[i] = validatedDate
		}
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	rUUID := project.TrimSpaceAndNonPrintable(request.UUID)
	if rUUID != "" {
		if code, err := realm.FindVerificationCodeByUUID(c.db, request.UUID); err != nil {
			if !database.IsNotFound(err) {
				return &issueResult{
					obsBlame:    observability.BlameServer,
					obsResult:   observability.ResultError("FAILED_TO_CHECK_UUID"),
					httpCode:    http.StatusInternalServerError,
					errorReturn: api.Error(err),
				}
			}
		} else if code != nil {
			return &issueResult{
				obsBlame:    observability.BlameClient,
				obsResult:   observability.ResultError("UUID_CONFLICT"),
				httpCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
	}

	// If we got this far, we're about to issue a code - take from the limiter
	// to ensure this is permitted.
	if realm.AbusePreventionEnabled {
		key, err := realm.QuotaKey(c.config.GetRateLimitConfig().HMACKey)
		if err != nil {
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_GENERATE_HMAC"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Error(err),
			}
		}
		limit, _, reset, ok, err := c.limiter.Take(ctx, key)
		if err != nil {
			logger.Errorw("failed to take from limiter", "error", err)
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_TAKE_FROM_LIMITER"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to verify realm stats, please try again"),
			}
		}

		stats.Record(ctx, mRealmTokenUsed.M(1))

		if !ok {
			logger.Warnw("realm has exceeded daily quota",
				"realm", realm.ID,
				"limit", limit,
				"reset", reset)

			if c.config.GetEnforceRealmQuotas() {
				return &issueResult{
					obsBlame:    observability.BlameClient,
					obsResult:   observability.ResultError("QUOTA_EXCEEDED"),
					httpCode:    http.StatusTooManyRequests,
					errorReturn: api.Errorf("exceeded realm quota, please contact a realm administrator").WithCode(api.ErrQuotaExceeded),
				}
			}
		}
	}

	now := time.Now().UTC()
	expiryTime := now.Add(realm.CodeDuration.Duration)
	longExpiryTime := now.Add(realm.LongCodeDuration.Duration)
	if request.Phone == "" || smsProvider == nil {
		// If this isn't going to be send via SMS, make the long code expiration time same as short.
		// This is because the long code will never be shown or sent.
		longExpiryTime = expiryTime
	}

	// Compute issuing user - the membership will be nil when called via the API.
	var user *database.User
	if membership != nil {
		user = membership.User
	}

	// Generate verification code
	codeRequest := otp.Request{
		DB:             c.db,
		ShortLength:    realm.CodeLength,
		ShortExpiresAt: expiryTime,
		LongLength:     realm.LongCodeLength,
		LongExpiresAt:  longExpiryTime,
		TestType:       request.TestType,
		SymptomDate:    parsedDates[0],
		TestDate:       parsedDates[1],
		MaxSymptomAge:  c.config.GetAllowedSymptomAge(),
		RealmID:        realm.ID,
		UUID:           rUUID,

		IssuingUser:       user,
		IssuingApp:        authApp,
		IssuingExternalID: request.ExternalIssuerID,
	}

	verCode, err := codeRequest.Issue(ctx, c.config.GetCollisionRetryCount())
	if err != nil {
		logger.Errorw("failed to issue code", "error", err)
		// GormV1 doesn't have a good way to match db errors
		if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
				httpCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
		return &issueResult{
			obsBlame:    observability.BlameServer,
			obsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
			httpCode:    http.StatusInternalServerError,
			errorReturn: api.Errorf("failed to generate otp code, please try again"),
		}
	}

	return &issueResult{
		verCode:   verCode,
		httpCode:  http.StatusOK,
		obsBlame:  observability.BlameNone,
		obsResult: observability.ResultOK(),
	}
}
