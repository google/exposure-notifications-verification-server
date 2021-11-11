// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
)

// BuildVerificationCode populates and validates a code from an issue request.
func (c *Controller) BuildVerificationCode(ctx context.Context, internalRequest *IssueRequestInternal, realm *database.Realm) (*database.VerificationCode, *IssueResult) {
	logger := logging.FromContext(ctx).Named("issueapi.buildVerificationCode")

	now := time.Now().UTC()
	request := internalRequest.IssueRequest
	vCode := &database.VerificationCode{
		RealmID:           realm.ID,
		IssuingExternalID: request.ExternalIssuerID,
		TestType:          strings.ToLower(request.TestType),
		ExpiresAt:         now.Add(realm.CodeDuration.Duration),
		LongExpiresAt:     now.Add(realm.LongCodeDuration.Duration),
	}
	if membership := controller.MembershipFromContext(ctx); membership != nil {
		vCode.IssuingUserID = membership.UserID
	}
	if authApp := controller.AuthorizedAppFromContext(ctx); authApp != nil {
		vCode.IssuingAppID = authApp.ID
	}

	// If this realm requires a date but no date was specified, return an error.
	if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
		return nil, &IssueResult{
			obsResult:   enobs.ResultError("MISSING_REQUIRED_FIELDS"),
			HTTPCode:    http.StatusBadRequest,
			ErrorReturn: api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate),
		}
	}

	// Parse SymptomDate and TestDate
	var result *IssueResult
	vCode.SymptomDate, result = c.parseDate(request.SymptomDate, int(request.TZOffset), &onsetSettings)
	if result != nil {
		return nil, result
	}
	vCode.TestDate, result = c.parseDate(request.TestDate, int(request.TZOffset), &testSettings)
	if result != nil {
		return nil, result
	}

	// Parse and canonicalize phone numbers.
	if request.Phone != "" {
		canonicalPhone, err := project.CanonicalPhoneNumber(request.Phone, realm.SMSCountry)
		if err != nil {
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("INVALID_PHONE"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Error(err).WithCode(api.ErrPhoneNumberInvalid),
			}
		}
		request.Phone = canonicalPhone
	}

	if request.OnlyGenerateSMS {
		if !realm.AllowGeneratedSMS {
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("ONLY_GENERATE_SMS_NOT_ALLOWED"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Errorf("realm is not permitted to use onlyGenerateSMS").WithCode(api.ErrUnparsableRequest),
			}
		}

		if request.Phone == "" {
			err := fmt.Errorf("generated sms requested, but no phone number was provided")
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("INVALID_GENERATE_SMS_REQUEST"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Error(err),
			}
		}
	}

	// Verify SMS configuration if phone was provided
	var smsProvider sms.Provider
	if !request.OnlyGenerateSMS && request.Phone != "" {
		var err error
		smsProvider, err = realm.SMSProvider(c.db)
		if err != nil {
			logger.Errorw("failed to get sms provider", "error", err)
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusInternalServerError,
				ErrorReturn: api.Errorf("failed to get sms provider").WithCode(api.ErrInternal),
			}
		}
		if smsProvider == nil {
			err := fmt.Errorf("phone provided, but no sms provider is configured")
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("FAILED_TO_GET_SMS_PROVIDER"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Error(err),
			}
		}
	}

	if request.Phone == "" || (smsProvider == nil && !request.OnlyGenerateSMS) {
		// If this isn't going to be send via SMS, make the long code expiration time same as short.
		// This is because the long code will never be shown or sent.
		vCode.LongExpiresAt = vCode.ExpiresAt
	}

	// If there is a client-provided UUID, check if a code has already been issued.
	// this prevents us from consuming quota on conflict.
	if vCode.UUID = project.TrimSpaceAndNonPrintable(request.UUID); vCode.UUID != "" {
		if code, err := realm.FindVerificationCodeByUUID(c.db, vCode.UUID); err != nil {
			if !database.IsNotFound(err) {
				return nil, &IssueResult{
					obsResult:   enobs.ResultError("FAILED_TO_CHECK_UUID"),
					HTTPCode:    http.StatusInternalServerError,
					ErrorReturn: api.Error(err).WithCode(api.ErrInternal),
				}
			}
		} else if code != nil {
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("UUID_CONFLICT"),
				HTTPCode:    http.StatusConflict,
				ErrorReturn: api.Errorf("code for %s already exists", vCode.UUID).WithCode(api.ErrUUIDAlreadyExists),
			}
		}
	}

	// Validation specific to PHA issued codes via API for user-report type.
	// This is a special case that needs to be opted in to.
	if request.TestType == verifyapi.ReportTypeSelfReport {
		// Make sure this realm allows for admin issued self reports.
		realm := controller.RealmFromContext(ctx)
		if !internalRequest.UserRequested && !realm.AllowAdminUserReport {
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("UNSUPPORTED_TEST_TYPE"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
			}
		}
	}

	vCode.Code = "placeholder"
	vCode.LongCode = "placeholder"
	if err := vCode.Validate(realm); err != nil {
		switch {
		case errors.Is(err, database.ErrInvalidTestType):
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("INVALID_TEST_TYPE"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Errorf("invalid test type").WithCode(api.ErrInvalidTestType),
			}
		case errors.Is(err, database.ErrUnsupportedTestType):
			return nil, &IssueResult{
				obsResult:   enobs.ResultError("UNSUPPORTED_TEST_TYPE"),
				HTTPCode:    http.StatusBadRequest,
				ErrorReturn: api.Errorf("unsupported test type: %v", request.TestType).WithCode(api.ErrUnsupportedTestType),
			}
		}

		logger.Warnw("unhandled db validation", "error", err)
		return nil, &IssueResult{
			obsResult:   enobs.ResultError("DB_VALIDATION_REJECTED"),
			HTTPCode:    http.StatusInternalServerError,
			ErrorReturn: api.Error(err).WithCode(api.ErrInternal),
		}
	}

	return vCode, nil
}
