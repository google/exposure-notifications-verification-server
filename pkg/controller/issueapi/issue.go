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
	"errors"
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

type dateParseSettings struct {
	Name          string
	ParseError    string
	ValidateError string
}

var (
	// Cache the UTC time.Location, to speed runtime.
	utc *time.Location

	onsetSettings = dateParseSettings{
		Name:          "symptom onset",
		ParseError:    "FAILED_TO_PROCESS_SYMPTOM_ONSET_DATE",
		ValidateError: "SYMPTOM_ONSET_DATE_NOT_IN_VALID_RANGE",
	}
	testSettings = dateParseSettings{
		Name:          "test",
		ParseError:    "FAILED_TO_PROCESS_TEST_DATE",
		ValidateError: "TEST_DATE_NOT_IN_VALID_RANGE",
	}
)

func init() {
	var err error
	utc, err = time.LoadLocation("UTC")
	if err != nil {
		panic("should have found UTC")
	}
}

// validateDate validates the date given -- returning the time or an error.
func validateDate(date, minDate, maxDate time.Time, tzOffset int) (*time.Time, error) {
	// Check that all our dates are utc.
	if date.Location() != utc || minDate.Location() != utc || maxDate.Location() != utc {
		return nil, errors.New("dates weren't in UTC")
	}

	// If we're dealing with a timezone where the offset is earlier than this one,
	// we loosen up the lower bound. We might have the following circumstance:
	//
	//    Server time: UTC Aug 1, 12:01 AM
	//    Client time: UTC July 30, 11:01 PM (ie, tzOffset = -30)
	//
	// In this circumstance, we'll have the following:
	//
	//    minTime: UTC July 31, maxTime: Aug 1, clientTime: July 30.
	//
	// which would be an error. Loosening up the lower bound, by a day, keeps us
	// all ok.
	if tzOffset < 0 {
		if m := minDate.Add(-24 * time.Hour); m.After(date) {
			return nil, fmt.Errorf("date %v before min %v", date, m)
		}
	} else if minDate.After(date) {
		return nil, fmt.Errorf("date %v before min %v", date, minDate)
	}
	if date.After(maxDate) {
		return nil, fmt.Errorf("date %v after max %v", date, maxDate)
	}
	return &date, nil
}

func (c *Controller) HandleIssue() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		logger := logging.FromContext(ctx).Named("issueapi.HandleIssue")

		var blame = observability.BlameNone
		var result = observability.ResultOK()
		defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &blame, &result)

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			blame = observability.BlameClient
			result = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")
			return
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			blame = observability.BlameClient
			result = observability.ResultError("MISSING_AUTHORIZED_APP")
			return
		}

		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.Realm(c.db)
			if err != nil {
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				blame = observability.BlameClient
				result = observability.ResultError("UNAUTHORIZED")
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
		}
		if realm == nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
			blame = observability.BlameServer
			result = observability.ResultError("MISSING_REALM")
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		ctx = observability.WithRealmID(ctx, realm.ID)

		// If this realm requires a date but no date was specified, return an error.
		if realm.RequireDate && request.SymptomDate == "" && request.TestDate == "" {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate))
			blame = observability.BlameClient
			result = observability.ResultError("MISSING_REQUIRED_FIELDS")
			return
		}

		// Validate that the request with the provided test type is valid for this realm.
		if !realm.ValidTestType(request.TestType) {
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("unsupported test type: %v", request.TestType))
			blame = observability.BlameClient
			result = observability.ResultError("UNSUPPORTED_TEST_TYPE")
			return
		}

		// Verify SMS configuration if phone was provided
		var smsProvider sms.Provider
		if request.Phone != "" {
			smsProvider, err = realm.SMSProvider(c.db)
			if err != nil {
				logger.Errorw("failed to get sms provider", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to get sms provider"))
				blame = observability.BlameServer
				result = observability.ResultError("FAILED_TO_GET_SMS_PROVIDER")
				return
			}
			if smsProvider == nil {
				err := fmt.Errorf("phone provided, but no sms provider is configured")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
				blame = observability.BlameServer
				result = observability.ResultError("FAILED_TO_GET_SMS_PROVIDER")
				return
			}
		}

		// Verify the test type
		request.TestType = strings.ToLower(request.TestType)
		if _, ok := c.validTestType[request.TestType]; !ok {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("invalid test type"))
			blame = observability.BlameClient
			result = observability.ResultError("INVALID_TEST_TYPE")
			return
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
					c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("failed to process %s date: %v", dateSettings[i].Name, err))
					blame = observability.BlameClient
					result = observability.ResultError(dateSettings[i].ParseError)
					return
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
					c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
					blame = observability.BlameClient
					result = observability.ResultError(dateSettings[i].ValidateError)
					return
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
					controller.InternalError(w, r, c.h, err)
					return
				}
			} else if code != nil {
				c.h.RenderJSON(w, http.StatusConflict,
					api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists))
				return
			}
		}

		// If we got this far, we're about to issue a code - take from the limiter
		// to ensure this is permitted.
		if realm.AbusePreventionEnabled {
			key, err := realm.QuotaKey(c.config.GetRateLimitConfig().HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				blame = observability.BlameServer
				result = observability.ResultError("FAILED_TO_GENERATE_HMAC")
				return
			}
			limit, _, reset, ok, err := c.limiter.Take(ctx, key)
			if err != nil {
				logger.Errorw("failed to take from limiter", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to verify realm stats, please try again"))
				blame = observability.BlameServer
				result = observability.ResultError("FAILED_TO_TAKE_FROM_LIMITER")
				return
			}

			stats.Record(ctx, mRealmTokenUsed.M(1))

			if !ok {
				logger.Warnw("realm has exceeded daily quota",
					"realm", realm.ID,
					"limit", limit,
					"reset", reset)

				if c.config.GetEnforceRealmQuotas() {
					c.h.RenderJSON(w, http.StatusTooManyRequests, api.Errorf("exceeded realm quota, please contact the realm admin."))
					blame = observability.BlameClient
					result = observability.ResultError("QUOTA_EXCEEDED")
					return
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
			IssuingUser:    user,
			IssuingApp:     authApp,
			RealmID:        realm.ID,
			UUID:           rUUID,
		}

		code, longCode, uuid, err := codeRequest.Issue(ctx, c.config.GetCollisionRetryCount())
		if err != nil {
			logger.Errorw("failed to issue code", "error", err)
			blame = observability.BlameServer
			result = observability.ResultError("FAILED_TO_ISSUE_CODE")

			// GormV1 doesn't have a good way to match db errors
			if strings.Contains(err.Error(), database.VercodeUUIDUniqueIndex) {
				c.h.RenderJSON(w, http.StatusConflict,
					api.Errorf("code for %s already exists", request.UUID).WithCode(api.ErrUUIDAlreadyExists))
				return
			}
			c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to generate otp code, please try again"))
			return
		}

		if request.Phone != "" && smsProvider != nil {
			if err := func() error {
				defer observability.RecordLatency(ctx, time.Now(), mSMSLatencyMs, &blame, &result)
				message := realm.BuildSMSText(code, longCode, c.config.GetENXRedirectDomain())

				if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
					// Delete the token
					if err := c.db.DeleteVerificationCode(code); err != nil {
						logger.Errorw("failed to delete verification code", "error", err)
						// fallthrough to the error
					}

					logger.Errorw("failed to send sms", "error", err)
					blame = observability.BlameServer
					result = observability.ResultError("FAILED_TO_SEND_SMS")
					return err
				}

				return nil
			}(); err != nil {
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to send sms: %s", err))
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.IssueCodeResponse{
				UUID:                   uuid,
				VerificationCode:       code,
				ExpiresAt:              expiryTime.Format(time.RFC1123),
				ExpiresAtTimestamp:     expiryTime.UTC().Unix(),
				LongExpiresAt:          longExpiryTime.Format(time.RFC1123),
				LongExpiresAtTimestamp: longExpiryTime.UTC().Unix(),
			})
	})
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
