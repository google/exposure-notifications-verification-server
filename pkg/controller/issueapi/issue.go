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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/digest"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"

	"go.opencensus.io/stats"
)

// Cache the UTC time.Location, to speed runtime.
var utc *time.Location

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
	logger := c.logger.Named("issueapi.HandleIssue")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithBuildInfo(r.Context())

		// Record the issue attempt.
		stats.Record(ctx, mIssueAttempts.M(1))

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			stats.Record(ctx, mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Use the symptom onset date if given, otherwise fallback to test date.
		if request.SymptomDate == "" {
			request.SymptomDate = request.TestDate
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			stats.Record(ctx, mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.Realm(c.db)
			if err != nil {
				stats.Record(ctx, mCodeIssueErrors.M(1))
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
		}
		if realm == nil {
			stats.Record(ctx, mIssueAttempts.M(1), mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
			return
		}

		// Add realm so that metrics are groupable on a per-realm basis.
		ctx = observability.WithRealmID(ctx, realm.ID)

		// If this realm requires a date but no date was specified, return an error.
		if request.SymptomDate == "" && realm.RequireDate {
			stats.Record(ctx, mIssueAttempts.M(1), mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing either test or symptom date").WithCode(api.ErrMissingDate))
			return
		}

		// Validate that the request with the provided test type is valid for this
		// realm.
		if !realm.ValidTestType(request.TestType) {
			stats.Record(ctx, mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest,
				api.Errorf("unsupported test type: %v", request.TestType))
			return
		}

		// Verify SMS configuration if phone was provided
		var smsProvider sms.Provider
		if request.Phone != "" {
			smsProvider, err = realm.SMSProvider(c.db)
			if err != nil {
				logger.Errorw("failed to get sms provider", "error", err)
				stats.Record(ctx, mCodeIssueErrors.M(1))
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to get sms provider"))
				return
			}
			if smsProvider == nil {
				err := fmt.Errorf("phone provided, but no sms provider is configured")
				stats.Record(ctx, mCodeIssueErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
				return
			}
		}

		// Verify the test type
		request.TestType = strings.ToLower(request.TestType)
		if _, ok := c.validTestType[request.TestType]; !ok {
			stats.Record(ctx, mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("invalid test type"))
			return
		}

		var symptomDate *time.Time
		if request.SymptomDate != "" {
			if parsed, err := time.Parse("2006-01-02", request.SymptomDate); err != nil {
				stats.Record(ctx, mCodeIssueErrors.M(1))
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("failed to process symptom onset date: %v", err))
				return
			} else {
				// Max date is today (UTC time) and min date is AllowedTestAge ago, truncated.
				maxDate := time.Now().UTC().Truncate(24 * time.Hour)
				minDate := maxDate.Add(-1 * c.config.GetAllowedSymptomAge()).Truncate(24 * time.Hour)

				symptomDate, err = validateDate(parsed, minDate, maxDate, int(request.TZOffset))
				if err != nil {
					err := fmt.Errorf("symptom onset date must be on/after %v and on/before %v %v",
						minDate.Format("2006-01-02"),
						maxDate.Format("2006-01-02"),
						parsed.Format("2006-01-02"),
					)
					stats.Record(ctx, mCodeIssueErrors.M(1))
					c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
					return
				}
			}
		}

		// If we got this far, we're about to issue a code - take from the limiter
		// to ensure this is permitted.
		if realm.AbusePreventionEnabled {
			dig, err := digest.HMACUint(realm.ID, c.config.GetRateLimitConfig().HMACKey)
			if err != nil {
				controller.InternalError(w, r, c.h, err)
				return
			}
			key := fmt.Sprintf("realm:quota:%s", dig)
			limit, remaining, reset, ok, err := c.limiter.Take(ctx, key)
			c.recordCapacity(ctx, limit, remaining)
			if err != nil {
				logger.Errorw("failed to take from limiter", "error", err)
				stats.Record(ctx, mQuotaErrors.M(1))
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to verify realm stats, please try again"))
				return
			}
			if !ok {
				logger.Warnw("realm has exceeded daily quota",
					"realm", realm.ID,
					"limit", limit,
					"reset", reset)
				stats.Record(ctx, mQuotaExceeded.M(1))

				if c.config.GetEnforceRealmQuotas() {
					c.h.RenderJSON(w, http.StatusTooManyRequests, api.Errorf("exceeded realm quota"))
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
			SymptomDate:    symptomDate,
			MaxSymptomAge:  c.config.GetAllowedSymptomAge(),
			IssuingUser:    user,
			IssuingApp:     authApp,
			RealmID:        realm.ID,
		}

		code, longCode, uuid, err := codeRequest.Issue(ctx, c.config.GetCollisionRetryCount())
		if err != nil {
			logger.Errorw("failed to issue code", "error", err)
			stats.Record(ctx, mCodeIssueErrors.M(1))
			c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to generate otp code, please try again"))
			return
		}

		if request.Phone != "" && smsProvider != nil {
			message := realm.BuildSMSText(code, longCode, c.config.GetENXRedirectDomain())
			if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
				// Delete the token
				if err := c.db.DeleteVerificationCode(code); err != nil {
					logger.Errorw("failed to delete verification code", "error", err)
					// fallthrough to the error
				}

				logger.Errorw("failed to send sms", "error", err)
				stats.Record(ctx, mCodeIssueErrors.M(1), mSMSSendErrors.M(1))
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to send sms"))
				return
			}
			stats.Record(ctx, mSMSSent.M(1))
		}

		stats.Record(ctx, mCodesIssued.M(1))
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

func (c *Controller) recordCapacity(ctx context.Context, limit, remaining uint64) {
	stats.Record(ctx, mRealmTokenRemaining.M(int64(remaining)))

	issued := uint64(limit) - remaining
	stats.Record(ctx, mRealmTokenIssued.M(int64(issued)))

	capacity := float64(issued) / float64(limit)
	stats.Record(ctx, mRealmTokenCapacity.M(capacity))
}
