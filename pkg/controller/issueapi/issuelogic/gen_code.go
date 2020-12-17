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
	verCode *database.VerificationCode

	HTTPCode    int
	ErrorReturn *api.ErrorReturn
	ObsBlame    tag.Mutator
	ObsResult   tag.Mutator
}

func (result *IssueResult) issueCodeResponse() *api.IssueCodeResponse {
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

func (c *Controller) IssueOne(ctx context.Context, request *api.IssueCodeRequest) (*IssueResult, *api.IssueCodeResponse) {
	logger := logging.FromContext(ctx).Named("issueapi.issueOne")

	// Generate code
	result := c.generateCode(ctx, request)
	if result.ErrorReturn != nil {
		return result, &api.IssueCodeResponse{
			ErrorCode: result.ErrorReturn.ErrorCode,
			Error:     result.ErrorReturn.Error,
		}
	}

	// Send SMS messages
	if err := c.sendSMS(ctx, request, result); err != nil {
		logger.Warnw("failed to send SMS", "error", err)
	}

	if result.ErrorReturn != nil {
		return result, &api.IssueCodeResponse{
			ErrorCode: result.ErrorReturn.ErrorCode,
			Error:     result.ErrorReturn.Error,
		}
	}

	// Convert to API response
	return result, result.issueCodeResponse()
}

func (c *Controller) IssueMany(ctx context.Context, requests []*api.IssueCodeRequest) ([]*IssueResult, []*api.IssueCodeResponse) {
	logger := logging.FromContext(ctx).Named("issueapi.issueMany")

	// Generate codes
	results := make([]*IssueResult, len(requests))
	for i, singleReq := range requests {
		results[i] = c.generateCode(ctx, singleReq)
	}

	// Send SMS messages
	var wg sync.WaitGroup
	for i, result := range results {
		if result.ErrorReturn != nil {
			continue
		}

		wg.Add(1)
		go func(request *api.IssueCodeRequest, r *IssueResult) {
			defer wg.Done()
			if err := c.sendSMS(ctx, request, r); err != nil {
				logger.Warnw("failed to send SMS", "error", err)
			}
		}(requests[i], result)
	}

	wg.Wait() // wait the SMS work group to finish

	// Convert to API response
	responses := make([]*api.IssueCodeResponse, len(requests))
	for i, result := range results {
		if result.ErrorReturn != nil {
			responses[i] = &api.IssueCodeResponse{
				ErrorCode: result.ErrorReturn.ErrorCode,
				Error:     result.ErrorReturn.Error,
			}
		} else {
			responses[i] = result.issueCodeResponse()
		}
	}
	return results, responses
}

// generateCode issues a code.
// Footgun: Does not send SMS messages
func (c *Controller) generateCode(ctx context.Context, request *api.IssueCodeRequest) *IssueResult {
	logger := logging.FromContext(ctx).Named("issueapi.generateCode")

	// If this realm requires a date but no date was specified, return an error.
	if c.realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !c.realm.ValidTestType(request.TestType) {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify the test type
	request.TestType = strings.ToLower(request.TestType)
	if _, ok := validTestType[request.TestType]; !ok {
		return &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Errorf("invalid test type").WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err := c.realm.SMSProvider(c.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Errorf("failed to get sms provider"),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Error(err),
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
					ErrorReturn: api.Errorf("failed to process %s date: %v", dateSettings[i].Name, err).WithCode(api.ErrUnparsableRequest),
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
				return &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError(dateSettings[i].ValidateError),
					HTTPCode:    http.StatusBadRequest,
					ErrorReturn: api.Error(err),
				}
			}
			parsedDates[i] = validatedDate
		}
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	rUUID := project.TrimSpaceAndNonPrintable(request.UUID)
	if rUUID != "" {
		if code, err := c.realm.FindVerificationCodeByUUID(c.db, request.UUID); err != nil {
			if !database.IsNotFound(err) {
				return &IssueResult{
					ObsBlame:    observability.BlameServer,
					ObsResult:   observability.ResultError("FAILED_TO_CHECK_UUID"),
					HTTPCode:    http.StatusInternalServerError,
					ErrorReturn: api.Error(err),
				}
			}
		} else if code != nil {
			return &IssueResult{
				ObsBlame:    observability.BlameClient,
				ObsResult:   observability.ResultError("UUID_CONFLICT"),
				HTTPCode:    http.StatusConflict,
				ErrorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
	}

	// If we got this far, we're about to issue a code - take from the limiter
	// to ensure this is permitted.
	if c.realm.AbusePreventionEnabled {
		key, err := c.realm.QuotaKey(c.config.GetRateLimitConfig().HMACKey)
		if err != nil {
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GENERATE_HMAC"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Error(err),
			}
		}
		limit, _, reset, ok, err := c.limiter.Take(ctx, key)
		if err != nil {
			logger.Errorw("failed to take from limiter", "error", err)
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_TAKE_FROM_LIMITER"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Errorf("failed to verify realm stats, please try again"),
			}
		}

		stats.Record(ctx, issuemetric.RealmTokenUsed.M(1))

		if !ok {
			logger.Warnw("realm has exceeded daily quota",
				"realm", c.realm.ID,
				"limit", limit,
				"reset", reset)

			if c.config.GetEnforceRealmQuotas() {
				return &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError("QUOTA_EXCEEDED"),
					HTTPCode:    http.StatusTooManyRequests,
					ErrorReturn: api.Errorf("exceeded realm quota, please contact a realm administrator").WithCode(api.ErrQuotaExceeded),
				}
			}
		}
	}

	now := time.Now().UTC()
	expiryTime := now.Add(c.realm.CodeDuration.Duration)
	longExpiryTime := now.Add(c.realm.LongCodeDuration.Duration)
	if request.Phone == "" || smsProvider == nil {
		// If this isn't going to be send via SMS, make the long code expiration time same as short.
		// This is because the long code will never be shown or sent.
		longExpiryTime = expiryTime
	}

	// Compute issuing user - the membership will be nil when called via the API.
	var user *database.User
	if c.membership != nil {
		user = c.membership.User
	}

	// Generate verification code
	codeRequest := otp.Request{
		DB:             c.db,
		ShortLength:    c.realm.CodeLength,
		ShortExpiresAt: expiryTime,
		LongLength:     c.realm.LongCodeLength,
		LongExpiresAt:  longExpiryTime,
		TestType:       request.TestType,
		SymptomDate:    parsedDates[0],
		TestDate:       parsedDates[1],
		MaxSymptomAge:  c.config.GetAllowedSymptomAge(),
		RealmID:        c.realm.ID,
		UUID:           rUUID,

		IssuingUser:       user,
		IssuingApp:        c.authApp,
		IssuingExternalID: request.ExternalIssuerID,
	}

	verCode, err := codeRequest.Issue(ctx, c.config.GetCollisionRetryCount())
	if err != nil {
		logger.Errorw("failed to issue code", "error", err)
		// GormV1 doesn't have a good way to match db errors
		if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
			return &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
				HTTPCode:    http.StatusConflict,
				ErrorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
		return &IssueResult{
			ObsBlame:    observability.BlameServer,
			ObsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
			HTTPCode:    http.StatusInternalServerError,
			ErrorReturn: api.Errorf("failed to generate otp code, please try again"),
		}
	}

	return &IssueResult{
		verCode:   verCode,
		HTTPCode:  http.StatusOK,
		ObsBlame:  observability.BlameNone,
		ObsResult: observability.ResultOK(),
	}
}
