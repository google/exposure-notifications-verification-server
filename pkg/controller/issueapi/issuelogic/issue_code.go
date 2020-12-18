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

package issuelogic

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller/issueapi/issuemetric"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

var (
	validTestType = map[string]struct{}{
		api.TestTypeConfirmed: {},
		api.TestTypeLikely:    {},
		api.TestTypeNegative:  {},
	}
)

type IssueResult struct {
	verCode     *database.VerificationCode
	errorReturn *api.ErrorReturn

	HTTPCode  int
	ObsBlame  tag.Mutator
	ObsResult tag.Mutator
}

func (result *IssueResult) IssueCodeResponse() *api.IssueCodeResponse {
	if result.errorReturn != nil {
		return &api.IssueCodeResponse{
			ErrorCode: result.errorReturn.ErrorCode,
			Error:     result.errorReturn.Error,
		}
	}

	v := result.verCode
	return &api.IssueCodeResponse{
		UUID:                   v.UUID,
		VerificationCode:       v.Code,
		ExpiresAt:              v.ExpiresAt.Format(time.RFC1123),
		ExpiresAtTimestamp:     v.ExpiresAt.UTC().Unix(),
		LongExpiresAt:          v.LongExpiresAt.Format(time.RFC1123),
		LongExpiresAtTimestamp: v.LongExpiresAt.UTC().Unix(),
	}
}

func (il *IssueLogic) IssueOne(ctx context.Context, request *api.IssueCodeRequest) *IssueResult {
	results := il.IssueMany(ctx, []*api.IssueCodeRequest{request})
	return results[0]
}

func (il *IssueLogic) IssueMany(ctx context.Context, requests []*api.IssueCodeRequest) []*IssueResult {
	// Generate codes
	results := make([]*IssueResult, len(requests))
	for i, singleReq := range requests {
		results[i] = il.generateCode(ctx, singleReq)
	}

	// Send SMS messages
	var wg sync.WaitGroup
	for i, result := range results {
		if result.errorReturn != nil {
			continue
		}

		wg.Add(1)
		go func(request *api.IssueCodeRequest, r *IssueResult) {
			defer wg.Done()
			il.sendSMS(ctx, request, r)
		}(requests[i], result)
	}

	wg.Wait() // wait the SMS work group to finish

	return results
}

// generateCode issues a code. Does not send SMS messages.
func (il *IssueLogic) generateCode(ctx context.Context, request *api.IssueCodeRequest) *IssueResult {
	logger := logging.FromContext(ctx).Named("issueapi.generateCode")

	// If this realm requires a date but no date was specified, return an error.
	if il.realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Verify the test type
	request.TestType = strings.ToLower(request.TestType)
	if _, ok := validTestType[request.TestType]; !ok {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("invalid test type").WithCode(api.ErrInvalidTestType),
		}
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !il.realm.ValidTestType(request.TestType) {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err := il.realm.SMSProvider(il.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to get sms provider"),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusBadRequest,
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
				return &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError(dateSettings[i].ParseError),
					HTTPCode:    http.StatusBadRequest,
					errorReturn: api.Errorf("failed to process %s date: %v", dateSettings[i].Name, err).WithCode(api.ErrUnparsableRequest),
				}
			}
			// Max date is today (UTC time) and min date is AllowedTestAge ago, truncated.
			maxDate := timeutils.UTCMidnight(time.Now())
			minDate := timeutils.Midnight(maxDate.Add(-1 * il.config.GetAllowedSymptomAge()))

			validatedDate, err := validateDate(parsed, minDate, maxDate, int(request.TZOffset))
			if err != nil {
				err := fmt.Errorf("%s date must be on/after %v and on/before %v %v",
					dateSettings[i].Name,
					minDate.Format(project.RFC3339Date),
					maxDate.Format(project.RFC3339Date),
					parsed.Format(project.RFC3339Date),
				)
				return &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError(dateSettings[i].ValidateError),
					HTTPCode:    http.StatusBadRequest,
					errorReturn: api.Error(err).WithCode(api.ErrInvalidDate),
				}
			}
			parsedDates[i] = validatedDate
		}
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	rUUID := project.TrimSpaceAndNonPrintable(request.UUID)
	if rUUID != "" {
		if code, err := il.realm.FindVerificationCodeByUUID(il.db, request.UUID); err != nil {
			if !database.IsNotFound(err) {
				return &IssueResult{
					ObsBlame:    observability.BlameServer,
					ObsResult:   observability.ResultError("FAILED_TO_CHECK_UUID"),
					HTTPCode:    http.StatusInternalServerError,
					errorReturn: api.Error(err),
				}
			}
		} else if code != nil {
			return &IssueResult{
				ObsBlame:    observability.BlameClient,
				ObsResult:   observability.ResultError("UUID_CONFLICT"),
				HTTPCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
	}

	// If we got this far, we're about to issue a code - take from the limiter
	// to ensure this is permitted.
	if il.realm.AbusePreventionEnabled {
		key, err := il.realm.QuotaKey(il.config.GetRateLimitConfig().HMACKey)
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

		stats.Record(ctx, issuemetric.RealmTokenUsed.M(1))

		if !ok {
			logger.Warnw("realm has exceeded daily quota",
				"realm", il.realm.ID,
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

	now := time.Now().UTC()
	expiryTime := now.Add(il.realm.CodeDuration.Duration)
	longExpiryTime := now.Add(il.realm.LongCodeDuration.Duration)
	if request.Phone == "" || smsProvider == nil {
		// If this isn't going to be send via SMS, make the long code expiration time same as short.
		// This is because the long code will never be shown or sent.
		longExpiryTime = expiryTime
	}

	// Compute issuing user - the membership will be nil when called via the API.
	var user *database.User
	if il.membership != nil {
		user = il.membership.User
	}

	// Generate verification code
	codeRequest := otp.Request{
		DB:             il.db,
		ShortLength:    il.realm.CodeLength,
		ShortExpiresAt: expiryTime,
		LongLength:     il.realm.LongCodeLength,
		LongExpiresAt:  longExpiryTime,
		TestType:       request.TestType,
		SymptomDate:    parsedDates[0],
		TestDate:       parsedDates[1],
		MaxSymptomAge:  il.config.GetAllowedSymptomAge(),
		RealmID:        il.realm.ID,
		UUID:           rUUID,

		IssuingUser:       user,
		IssuingApp:        il.authApp,
		IssuingExternalID: request.ExternalIssuerID,
	}

	verCode, err := codeRequest.Issue(ctx, il.config.GetCollisionRetryCount())
	if err != nil {
		logger.Errorw("failed to issue code", "error", err)
		// GormV1 doesn't have a good way to match db errors
		if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
				HTTPCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
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
		verCode:   verCode,
		HTTPCode:  http.StatusOK,
		ObsBlame:  observability.BlameNone,
		ObsResult: observability.ResultOK(),
	}
}
