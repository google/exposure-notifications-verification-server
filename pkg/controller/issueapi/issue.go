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

// Package issueapi implements the API handler for taking a code requst, assigning
// an OTP, saving it to the database and returning the result.
// This is invoked over AJAX from the Web frontend.
package issueapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"

	httpcontext "github.com/gorilla/context"

	"go.uber.org/zap"
)

// IssueAPI is a controller for the verification code JSON API.
type IssueAPI struct {
	config        config.IssueAPIConfig
	db            *database.Database
	logger        *zap.SugaredLogger
	validTestType map[string]struct{}
}

// New creates a new IssueAPI controller.
func New(ctx context.Context, config config.IssueAPIConfig, db *database.Database) http.Handler {
	controller := IssueAPI{
		config:        config,
		db:            db,
		logger:        logging.FromContext(ctx),
		validTestType: make(map[string]struct{}),
	}
	// Set up the valid test type strings from the API definition.
	controller.validTestType[api.TestTypeConfirmed] = struct{}{}
	controller.validTestType[api.TestTypeLikely] = struct{}{}
	controller.validTestType[api.TestTypeNegative] = struct{}{}
	return &controller
}

func (ic *IssueAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request api.IssueCodeRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		ic.logger.Errorf("failed to bind request: %v", err)
		controller.WriteJSON(w, http.StatusOK, api.Error("invalid request: %v", err))
		return
	}

	// Verify the test type
	request.TestType = strings.ToLower(request.TestType)
	if _, ok := ic.validTestType[request.TestType]; !ok {
		ic.logger.Errorf("invalid test type: %v", request.TestType)
		controller.WriteJSON(w, http.StatusOK, api.Error("invalid test type: %v", request.TestType))
		return
	}

	// Max date is today (local time) and min date is AllowedTestAge ago, truncated.
	maxDate := time.Now().Local()
	minDate := maxDate.Add(-1 * ic.config.GetAllowedSymptomAge()).Truncate(24 * time.Hour)

	var symptomDate *time.Time
	if request.SymptomDate != "" {
		if parsed, err := time.Parse("2006-01-02", request.SymptomDate); err != nil {
			ic.logger.Errorf("time.Parse: %v", err)
			controller.WriteJSON(w, http.StatusOK, api.Error("invalid test date: %v", err))
			return
		} else {
			parsed = parsed.Local()
			if minDate.After(parsed) || parsed.After(maxDate) {
				message := fmt.Sprintf("Invalid symptom onset date: %v must be on or after %v and on or before %v.",
					parsed.Format("2006-01-02"), minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
				ic.logger.Errorf(message)
				controller.WriteJSON(w, http.StatusOK, api.Error(message))
				return
			}
			symptomDate = &parsed
		}
	}

	expiryTime := time.Now().Add(ic.config.GetVerificationCodeDuration())

	authApp, user, err := ic.getAuthorizationFromContext(r)
	if err != nil {
		ic.logger.Errorf("failed to get authorization: %v", err)
		controller.WriteJSON(w, http.StatusOK, api.Error("invalid request: %v", err))
		return
	}

	// Generate verification code
	codeRequest := otp.Request{
		DB:            ic.db,
		Length:        ic.config.GetVerficationCodeDigits(),
		ExpiresAt:     expiryTime,
		TestType:      request.TestType,
		SymptomDate:   symptomDate,
		MaxSymptomAge: ic.config.GetAllowedSymptomAge(),
		IssuingUser:   user,
		IssuingApp:    authApp,
	}

	code, err := codeRequest.Issue(ctx, ic.config.GetColissionRetryCount())
	if err != nil {
		ic.logger.Errorf("otp.GenerateCode: %v", err)
		controller.WriteJSON(w, http.StatusOK, api.Error("error generating verification, wait a moment and try again"))
		return
	}

	controller.WriteJSON(w, http.StatusOK,
		&api.IssueCodeResponse{
			VerificationCode: code,
			ExpiresAt:        expiryTime.Format(time.RFC1123),
		})
}

func (ic *IssueAPI) getAuthorizationFromContext(r *http.Request) (*database.AuthorizedApp, *database.User, error) {
	// Attempt to find the authorized app.
	rawAuthorizedApp, ok := httpcontext.GetOk(r, "authorizedApp")
	if ok {
		authApp, ok := rawAuthorizedApp.(*database.AuthorizedApp)
		if !ok {
			return nil, nil, fmt.Errorf("unable to identify authorized requestor")
		}
		return authApp, nil, nil
	}

	// Attempt to get user:
	rawUser, ok := httpcontext.GetOk(r, "user")
	if !ok {
		return nil, nil, fmt.Errorf("unable to identify authorized requestor")
	}
	user, ok := rawUser.(*database.User)
	if !ok {
		return nil, nil, fmt.Errorf("unable to identify authorized requestor")
	}
	return nil, user, nil

}
