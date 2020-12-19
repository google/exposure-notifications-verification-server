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
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/timeutils"
	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

var (
	validTestType = map[string]struct{}{
		api.TestTypeConfirmed: {},
		api.TestTypeLikely:    {},
		api.TestTypeNegative:  {},
	}
)

func (il *Controller) IssueOne(ctx context.Context, request *api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) *IssueResult {
	results := il.IssueMany(ctx, []*api.IssueCodeRequest{request}, authApp, membership, realm)
	return results[0]
}

func (il *Controller) IssueMany(ctx context.Context, requests []*api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) []*IssueResult {
	// Generate codes
	results := make([]*IssueResult, len(requests))
	for i, req := range requests {
		vCode, result := il.populateCode(ctx, req, authApp, membership, realm)
		if result != nil {
			results[i] = result
			continue
		}
		results[i] = il.issueCode(ctx, vCode, realm)
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
			il.sendSMS(ctx, request, r, realm)
		}(requests[i], result)
	}

	wg.Wait() // wait the SMS work group to finish

	return results
}

// populateCode populates a code from an issue request.
func (il *Controller) populateCode(ctx context.Context, request *api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) (*database.VerificationCode, *IssueResult) {
	logger := logging.FromContext(ctx).Named("issueapi.populateCode")

	vCode := &database.VerificationCode{
		RealmID:           realm.ID,
		IssuingExternalID: request.ExternalIssuerID,
		TestType:          request.TestType,
	}
	if membership != nil {
		if user := membership.User; user != nil {
			vCode.IssuingUserID = user.ID
		}
	}
	if authApp != nil {
		vCode.IssuingAppID = authApp.ID
	}

	// If this realm requires a date but no date was specified, return an error.
	if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return nil, &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Set up parallel arrays to leverage the observability reporting and connect the parse / validation errors
	// to the correct date.
	parsedDates := make([]*time.Time, 2)
	dateSettings := []*dateParseSettings{&onsetSettings, &testSettings}
	for i, d := range []string{request.SymptomDate, request.TestDate} {
		if d != "" {
			parsed, err := time.Parse(project.RFC3339Date, d)
			if err != nil {
				return nil, &IssueResult{
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
				return nil, &IssueResult{
					ObsBlame:    observability.BlameClient,
					ObsResult:   observability.ResultError(dateSettings[i].ValidateError),
					HTTPCode:    http.StatusBadRequest,
					errorReturn: api.Error(err).WithCode(api.ErrInvalidDate),
				}
			}
			parsedDates[i] = validatedDate
		}
	}
	vCode.SymptomDate = parsedDates[0]
	vCode.TestDate = parsedDates[1]

	// Verify the test type
	vCode.TestType = strings.ToLower(request.TestType)
	if _, ok := validTestType[request.TestType]; !ok {
		return nil, &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("invalid test type").WithCode(api.ErrInvalidTestType),
		}
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !realm.ValidTestType(vCode.TestType) {
		return nil, &IssueResult{
			ObsBlame:    observability.BlameClient,
			ObsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			HTTPCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err := realm.SMSProvider(il.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return nil, &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to get sms provider"),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return nil, &IssueResult{
				ObsBlame:    observability.BlameServer,
				ObsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusBadRequest,
				errorReturn: api.Error(err),
			}
		}
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	if rUUID := project.TrimSpaceAndNonPrintable(request.UUID); rUUID != "" {
		if code, err := realm.FindVerificationCodeByUUID(il.db, request.UUID); err != nil {
			if !database.IsNotFound(err) {
				return nil, &IssueResult{
					ObsBlame:    observability.BlameServer,
					ObsResult:   observability.ResultError("FAILED_TO_CHECK_UUID"),
					HTTPCode:    http.StatusInternalServerError,
					errorReturn: api.Error(err),
				}
			}
		} else if code != nil {
			return nil, &IssueResult{
				ObsBlame:    observability.BlameClient,
				ObsResult:   observability.ResultError("UUID_CONFLICT"),
				HTTPCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
		vCode.UUID = rUUID
	}

	now := time.Now().UTC()
	vCode.ExpiresAt = now.Add(realm.CodeDuration.Duration)
	vCode.LongExpiresAt = now.Add(realm.LongCodeDuration.Duration)
	if request.Phone == "" || smsProvider == nil {
		// If this isn't going to be send via SMS, make the long code expiration time same as short.
		// This is because the long code will never be shown or sent.
		vCode.LongExpiresAt = vCode.ExpiresAt
	}
	return vCode, nil
}
