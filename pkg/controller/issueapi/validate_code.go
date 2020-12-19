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

// populateCode populates and validates a code from an issue request.
func (c *Controller) populateCode(ctx context.Context, request *api.IssueCodeRequest,
	authApp *database.AuthorizedApp, membership *database.Membership, realm *database.Realm) (*database.VerificationCode, *issueResult) {
	logger := logging.FromContext(ctx).Named("issueapi.populateCode")

	vCode := &database.VerificationCode{
		RealmID:           realm.ID,
		IssuingExternalID: request.ExternalIssuerID,
		TestType:          request.TestType,
	}
	if membership != nil {
		vCode.IssuingUserID = membership.UserID
	}
	if authApp != nil {
		vCode.IssuingAppID = authApp.ID
	}

	// If this realm requires a date but no date was specified, return an error.
	if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return nil, &issueResult{
			obsResult:   observability.ResultError("MISSING_REQUIRED_FIELDS"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Parse and validate SymptomDate and TestDate
	var result *issueResult
	vCode.SymptomDate, result = c.parseDate(request.SymptomDate, int(request.TZOffset), &onsetSettings)
	if result != nil {
		return nil, result
	}
	vCode.TestDate, result = c.parseDate(request.TestDate, int(request.TZOffset), &testSettings)
	if result != nil {
		return nil, result
	}

	// Verify the test type
	vCode.TestType = strings.ToLower(request.TestType)
	if _, ok := validTestType[request.TestType]; !ok {
		return nil, &issueResult{
			obsResult:   observability.ResultError("INVALID_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("invalid test type").WithCode(api.ErrInvalidTestType),
		}
	}

	// Validate that the request with the provided test type is valid for this realm.
	if !realm.ValidTestType(vCode.TestType) {
		return nil, &issueResult{
			obsResult:   observability.ResultError("UNSUPPORTED_TEST_TYPE"),
			httpCode:    http.StatusBadRequest,
			errorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if request.Phone != "" {
		smsProvider, err := realm.SMSProvider(c.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return nil, &issueResult{
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusInternalServerError,
				errorReturn: api.Errorf("failed to get sms provider"),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return nil, &issueResult{
				obsResult:   observability.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				httpCode:    http.StatusBadRequest,
				errorReturn: api.Error(err),
			}
		}
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	rUUID := project.TrimSpaceAndNonPrintable(request.UUID)
	if rUUID != "" {
		if code, err := realm.FindVerificationCodeByUUID(c.db, request.UUID); err != nil {
			if !database.IsNotFound(err) {
				return nil, &issueResult{
					obsResult:   observability.ResultError("FAILED_TO_CHECK_UUID"),
					httpCode:    http.StatusInternalServerError,
					errorReturn: api.Error(err),
				}
			}
		} else if code != nil {
			return nil, &issueResult{
				obsResult:   observability.ResultError("UUID_CONFLICT"),
				httpCode:    http.StatusConflict,
				errorReturn: api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
	}
	vCode.UUID = rUUID

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
