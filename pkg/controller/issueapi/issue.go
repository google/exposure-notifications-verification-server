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

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// IssueAPI is a controller for the verification code JSON API.
type IssueAPI struct {
	config  *config.Config
	db      *database.Database
	session *controller.SessionHelper
	logger  *zap.SugaredLogger
}

// New creates a new IssueAPI controller.
func New(ctx context.Context, config *config.Config, db *database.Database, session *controller.SessionHelper) controller.Controller {
	return &IssueAPI{config, db, session, logging.FromContext(ctx)}
}

func (ic *IssueAPI) Execute(c *gin.Context) {
	response := api.IssueCodeResponse{}

	var request api.IssueCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ic.logger.Errorf("failed to bind request: %v", err)
		response.Error = fmt.Sprintf("invalid request: %v", err)
		c.JSON(http.StatusOK, response)
		return
	}

	var testDate *time.Time
	if request.TestDate != "" {
		if parsed, err := time.Parse("2006-01-02", request.TestDate); err != nil {
			ic.logger.Errorf("time.Parse: %v", err)
			response.Error = fmt.Sprintf("invalid test date: %v", err)
			c.JSON(http.StatusOK, response)
		} else {
			testDate = &parsed
		}
	}

	expiryTime := time.Now().Add(ic.config.CodeDuration)

	// Generate verification code
	codeRequest := otp.OTPRequest{
		DB:         ic.db,
		Length:     ic.config.CodeDigits,
		ExpiresAt:  expiryTime,
		TestType:   request.TestType,
		TestDate:   testDate,
		MaxTestAge: ic.config.AllowedTestAge,
	}

	code, err := codeRequest.Issue(c.Request.Context(), ic.config.ColissionRetryCount)
	if err != nil {
		ic.logger.Errorf("otp.GenerateCode: %v", err)
		response.Error = "error generating verification, wait a moment and try again"
		c.JSON(http.StatusOK, response)
		return
	}

	response.VerificationCode = code
	response.ExpiresAt = expiryTime.Format(time.RFC1123)
	c.JSON(http.StatusOK, response)
}
