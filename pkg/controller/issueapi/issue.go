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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/config"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/logging"
	"github.com/google/exposure-notifications-verification-server/pkg/otp"

	"go.uber.org/zap"
)

// IssueAPI is a controller for the verification code JSON API.
type IssueAPI struct {
	config *config.ServerConfig
	db     *database.Database
	logger *zap.SugaredLogger
}

// New creates a new IssueAPI controller.
func New(ctx context.Context, config *config.ServerConfig, db *database.Database) http.Handler {
	return &IssueAPI{config, db, logging.FromContext(ctx)}
}

func (ic *IssueAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request api.IssueCodeRequest
	if err := controller.BindJSON(w, r, &request); err != nil {
		ic.logger.Errorf("failed to bind request: %v", err)
		controller.WriteJSON(w, http.StatusOK, api.Error("invalid request: %v", err))
		return
	}

	// Max date is today (local time) and min date is AllowedTestAge ago, truncated.
	maxDate := time.Now().Local()
	minDate := maxDate.Add(-1 * ic.config.AllowedTestAge).Truncate(24 * time.Hour)

	var testDate *time.Time
	if request.TestDate != "" {
		if parsed, err := time.Parse("2006-01-02", request.TestDate); err != nil {
			ic.logger.Errorf("time.Parse: %v", err)
			controller.WriteJSON(w, http.StatusOK, api.Error("invalid test date: %v", err))
			return
		} else {
			parsed = parsed.Local()
			if minDate.After(parsed) || parsed.After(maxDate) {
				message := fmt.Sprintf("Invalid test date: %v must be on or after %v and on or before %v.",
					parsed.Format("2006-01-02"), minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
				ic.logger.Errorf(message)
				controller.WriteJSON(w, http.StatusOK, api.Error(message))
				return
			}
			testDate = &parsed
		}
	}

	expiryTime := time.Now().Add(ic.config.CodeDuration)

	// Generate verification code
	codeRequest := otp.Request{
		DB:         ic.db,
		Length:     ic.config.CodeDigits,
		ExpiresAt:  expiryTime,
		TestType:   request.TestType,
		TestDate:   testDate,
		MaxTestAge: ic.config.AllowedTestAge,
	}

	code, err := codeRequest.Issue(ctx, ic.config.CollisionRetryCount)
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
