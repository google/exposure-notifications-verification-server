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
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
	"go.opencensus.io/stats"
)

func (c *Controller) issue(ctx context.Context, request *api.IssueCodeRequest) (*issueResult, *api.IssueCodeResponse) {
	logger := logging.FromContext(ctx).Named("issueapi.issue")
	realm := controller.RealmFromContext(ctx)
	var err error

	// If this realm requires a date but no date was specified, return an error.
	if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}, nil
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !realm.ValidTestType(request.TestType) {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("unsupported test type: %v", request.TestType),
		}, nil
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err = realm.SMSProvider(c.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to get sms provider"),
			}, nil
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusBadRequest,
				errorReturn: api.Error(err),
			}, nil
		}
	}

	// Verify the test type
	request.TestType = strings.ToLower(request.TestType)
	if _, ok := c.validTestType[request.TestType]; !ok {
		return &issueResult{
			obsBlame:    observability.BlameClient,
			obsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("invalid test type"),
		}, nil
	}

	// Set up parallel arrays to leverage the observability reporting and connect the parse / validation errors
	// to the correct date.
	parsedDates := make([]*time.Time, 2)
	input := []string{request.SymptomDate, request.TestDate}
	dateSettings := []*dateParseSettings{&onsetSettings, &testSettings}
	for i, d := range input {
		if d != "" {
			parsed, err := time.Parse("2006-01-02", d)
			if err != nil {
				return &issueResult{
					obsBlame:    observability.BlameClient,
					obsResult:   observability.ResultError(dateSettings[i].ParseError),
					httpCode:    http.StatusBadRequest,
					errorReturn: api.Errorf("failed to process %s date: %v", dateSettings[i].Name, err),
				}, nil
			}
			// Max date is today (UTC time) and min date is AllowedTestAge ago, truncated.
			maxDate := timeutils.UTCMidnight(time.Now())
			minDate := timeutils.Midnight(maxDate.Add(-1 * c.config.GetAllowedSymptomAge()))

			validatedDate, err := validateDate(parsed, minDate, maxDate, int(request.TZOffset))
			if err != nil {
				err := fmt.Errorf("%s date must be on/after %v and on/before %v %v",
					dateSettings[i].Name,
					minDate.Format("2006-01-02"),
					maxDate.Format("2006-01-02"),
					parsed.Format("2006-01-02"),
				)
				return &issueResult{
					obsBlame:    observability.BlameClient,
					obsResult:   observability.ResultError(dateSettings[i].ValidateError),
					httpCode:    http.StatusBadRequest,
					errorReturn: api.Error(err),
				}, nil
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
				}, nil
			}
		} else if code != nil {
			return &issueResult{
				obsBlame:    observability.BlameClient,
				obsResult:   observability.ResultError("UUID_CONFLICT"),
				httpCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}, nil
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
			}, nil
		}
		limit, _, reset, ok, err := c.limiter.Take(ctx, key)
		if err != nil {
			logger.Errorw("failed to take from limiter", "error", err)
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_TAKE_FROM_LIMITER"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to verify realm stats, please try again"),
			}, nil
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
					errorReturn: api.Errorf("exceeded realm quota, please contact the realm admin."),
				}, nil
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

		IssuingUser:       controller.UserFromContext(ctx),
		IssuingApp:        controller.AuthorizedAppFromContext(ctx),
		IssuingExternalID: request.ExternalIssuerID,
	}

	code, longCode, uuid, err := codeRequest.Issue(ctx, c.config.GetCollisionRetryCount())
	if err != nil {
		logger.Errorw("failed to issue code", "error", err)
		// GormV1 doesn't have a good way to match db errors
		if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
			return &issueResult{
				obsBlame:    observability.BlameServer,
				obsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
				httpCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}, nil
		}
		return &issueResult{
			obsBlame:    observability.BlameServer,
			obsResult:   observability.ResultError("FAILED_TO_ISSUE_CODE"),
			httpCode:    http.StatusInternalServerError,
			errorReturn: api.Errorf("failed to generate otp code, please try again"),
		}, nil
	}

	result := &issueResult{
		httpCode:  http.StatusOK,
		obsBlame:  observability.BlameNone,
		obsResult: observability.ResultOK(),
	}

	if request.Phone != "" && smsProvider != nil {
		if err := func() error {
			defer observability.RecordLatency(&ctx, time.Now(), mSMSLatencyMs, &result.obsBlame, &result.obsResult)

			message := realm.BuildSMSText(code, longCode, c.config.GetENXRedirectDomain())

			if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
				// Delete the token
				if err := c.db.DeleteVerificationCode(code); err != nil {
					logger.Errorw("failed to delete verification code", "error", err)
					// fallthrough to the error
				}

				logger.Errorw("failed to send sms", "error", err)
				result.obsBlame = observability.BlameServer
				result.obsResult = observability.ResultError("FAILED_TO_SEND_SMS")
				return err
			}
			return nil
		}(); err != nil {
			result.httpCode = http.StatusInternalServerError
			result.errorReturn = api.Errorf("failed to send sms: %s", err)
			return result, nil
		}
	}

	return result, &api.IssueCodeResponse{
		UUID:                   uuid,
		VerificationCode:       code,
		ExpiresAt:              expiryTime.Format(time.RFC1123),
		ExpiresAtTimestamp:     expiryTime.UTC().Unix(),
		LongExpiresAt:          longExpiryTime.Format(time.RFC1123),
		LongExpiresAtTimestamp: longExpiryTime.UTC().Unix(),
	}
}

func (c *Controller) getAuthorizationFromContext(r *http.Request) (*database.AuthorizedApp, *database.User, error) {
	ctx := r.Context()

	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	currentUser := controller.UserFromContext(ctx)

	if authorizedApp == nil && currentUser == nil {
		return nil, nil, fmt.Errorf("unable to identify authorized requestor")
	}

	return authorizedApp, currentUser, nil
}

func recordObservability(ctx context.Context, result *issueResult) {
	observability.RecordLatency(&ctx, time.Now(), mLatencyMs, &result.obsBlame, &result.obsResult)
}
