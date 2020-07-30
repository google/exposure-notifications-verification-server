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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var request api.IssueCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			c.logger.Errorf("failed to bind request: %v", err)
			c.h.RenderJSON(w, http.StatusOK, api.Error("invalid request: %v", err))
			return
		}

		authApp, user, err := c.getAuthorizationFromContext(r)
		if err != nil {
			c.logger.Errorf("failed to get authorization: %v", err)
			c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error("invalid request: %v", err))
			return
		}
		var realm *database.Realm
		if authApp != nil {
			realm, err = authApp.GetRealm(c.db)
			if err != nil {
				c.h.RenderJSON(w, http.StatusUnauthorized, api.Error(http.StatusText(http.StatusUnauthorized)))
				return
			}
		} else {
			// if it's a user logged in, we can pull realm from the context.
			realm = controller.RealmFromContext(ctx)
			if realm == nil {
				c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error("no realm selected"))
				return
			}
		}

		// Verify SMS configuration if phone was provided
		var smsProvider sms.Provider
		if request.Phone != "" {
			smsProvider, err = realm.GetSMSProvider(ctx, c.db)
			if err != nil {
				c.logger.Errorf("GetSMSProvider: %v", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Error("failed to get sms provider"))
				return
			}
			if smsProvider == nil {
				err := fmt.Errorf("phone provided, but no SMS provider is configured")
				c.logger.Errorf("otp.GetSMSProvider: %v", err)
				c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error("%v", err))
			}
		}

		// Verify the test type
		request.TestType = strings.ToLower(request.TestType)
		if _, ok := c.validTestType[request.TestType]; !ok {
			c.logger.Errorf("invalid test type: %v", request.TestType)
			c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error("invalid test type: %v", request.TestType))
			return
		}

		// Max date is today (local time) and min date is AllowedTestAge ago, truncated.
		maxDate := time.Now().Local()
		minDate := maxDate.Add(-1 * c.config.GetAllowedSymptomAge()).Truncate(24 * time.Hour)

		var symptomDate *time.Time
		if request.SymptomDate != "" {
			if parsed, err := time.Parse("2006-01-02", request.SymptomDate); err != nil {
				c.logger.Errorf("time.Parse: %v", err)
				c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error("invalid symptom onset date: %v", err))
				return
			} else {
				parsed = parsed.Local()
				if minDate.After(parsed) || parsed.After(maxDate) {
					message := fmt.Sprintf("Invalid symptom onset date: %v must be on or after %v and on or before %v.",
						parsed.Format("2006-01-02"), minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
					c.logger.Errorf(message)
					c.h.RenderJSON(w, http.StatusUnprocessableEntity, api.Error(message))
					return
				}
				symptomDate = &parsed
			}
		}

		expiration := c.config.GetVerificationCodeDuration()
		expiryTime := time.Now().UTC().Add(expiration)

		// Generate verification code
		codeRequest := otp.Request{
			DB:            c.db,
			Length:        c.config.GetVerficationCodeDigits(),
			ExpiresAt:     expiryTime,
			TestType:      request.TestType,
			SymptomDate:   symptomDate,
			MaxSymptomAge: c.config.GetAllowedSymptomAge(),
			IssuingUser:   user,
			IssuingApp:    authApp,
			RealmID:       realm.ID,
		}

		code, err := codeRequest.Issue(ctx, c.config.GetColissionRetryCount())
		if err != nil {
			c.logger.Errorf("otp.GenerateCode: %v", err)
			c.h.RenderJSON(w, http.StatusInternalServerError, api.Error("error generating verification, wait a moment and try again"))
			return
		}

		if request.Phone != "" && smsProvider != nil {
			message := fmt.Sprintf(smsTemplate, code, int(expiration.Minutes()))
			if err := smsProvider.SendSMS(ctx, request.Phone, message); err != nil {
				// Delete the token
				if err := c.db.DeleteVerificationCode(code); err != nil {
					c.logger.Errorf("failed to delete verification code: %v")
					// fallthrough to the error
				}

				c.logger.Errorf("otp.SendSMS: %v", err)
				c.h.RenderJSON(w, http.StatusInternalServerError, api.Error("failed to send sms: %v", err))
				return
			}
		}

		c.h.RenderJSON(w, http.StatusOK,
			&api.IssueCodeResponse{
				VerificationCode:   code,
				ExpiresAt:          expiryTime.Format(time.RFC1123),
				ExpiresAtTimestamp: expiryTime.Unix(),
			})
	})
}

func (c *Controller) getAuthorizationFromContext(r *http.Request) (*database.AuthorizedApp, *database.User, error) {
	ctx := r.Context()

	// Attempt to find the authorized app.
	authorizedApp := controller.AuthorizedAppFromContext(ctx)
	if authorizedApp != nil {
		return authorizedApp, nil, nil
	}

	// Attempt to get user.
	user := controller.UserFromContext(ctx)
	if user != nil {
		return nil, user, nil
	}

	return nil, nil, fmt.Errorf("unable to identify authorized requestor")
}

const smsTemplate = `Your exposure notifications verification code is %s. ` +
	`Enter this code into your exposure notifications app. Do NOT share this ` +
	`code with anyone. This code expires in %d minutes.`
