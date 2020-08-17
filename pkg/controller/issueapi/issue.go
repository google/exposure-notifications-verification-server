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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"
	"github.com/google/exposure-notifications-verification-server/pkg/sms"
)

func (c *Controller) HandleIssue() http.Handler {
	logger := c.logger.Named("issueapi.HandleIssue")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			return
		}

		// Use the symptom onset date if given, otherwise fallback to test date.
		if request.SymptomDate == "" {
			request.SymptomDate = request.TestDate
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(err))
			return
		}

		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.Realm(c.db)
			if err != nil {
				c.h.RenderJSON(w, http.StatusUnauthorized, nil)
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
		}
		if realm == nil {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("missing realm"))
			return
		}

		// Validate that the request with the provided test type is valid for this
		// realm.
		if !realm.ValidTestType(request.TestType) {
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
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to get sms provider"))
				return
			}
			if smsProvider == nil {
				err := fmt.Errorf("phone provided, but no sms provider is configured")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
			}
		}

		// Verify the test type
		request.TestType = strings.ToLower(request.TestType)
		if _, ok := c.validTestType[request.TestType]; !ok {
			c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("invalid test type"))
			return
		}

		// Max date is today (local time) and min date is AllowedTestAge ago, truncated.
		maxDate := time.Now().Local()
		minDate := maxDate.Add(-1 * c.config.GetAllowedSymptomAge()).Truncate(24 * time.Hour)

		var symptomDate *time.Time
		if request.SymptomDate != "" {
			if parsed, err := time.Parse("2006-01-02", request.SymptomDate); err != nil {
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("failed to process symptom onset date: %v", err))
				return
			} else {
				parsed = parsed.Local()
				if minDate.After(parsed) || parsed.After(maxDate) {
					err := fmt.Errorf("symptom onset date must be on/after %v and on/before %v",
						minDate.Format("2006-01-02"),
						maxDate.Format("2006-01-02"))
					c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err))
					return
				}
				symptomDate = &parsed
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
			c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to generate otp code, please try again"))
			return
		}

		if request.Phone != "" && smsProvider != nil {
			message := realm.BuildSMSText(code, longCode)
			if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
				// Delete the token
				if err := c.db.DeleteVerificationCode(code); err != nil {
					logger.Errorw("failed to delete verification code", "error", err)
					// fallthrough to the error
				}

				logger.Errorw("failed to send sms", "error", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Errorf("failed to send sms"))
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.IssueCodeResponse{
				UUID:               uuid,
				VerificationCode:   code,
				ExpiresAt:          expiryTime.Format(time.RFC1123),
				ExpiresAtTimestamp: expiryTime.UTC().Unix(),
			})
	})
}

func (c *Controller) getAuthorizationFromContext(r *http.Request) (*database.AuthorizedApp, *database.User, error) {
	ctx := r.Context()

	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	user := controller.UserFromContext(ctx)

	if authorizedApp == nil && user == nil {
		return nil, nil, fmt.Errorf("unable to identify authorized requestor")
	}

	return authorizedApp, user, nil
}
