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

// Package verifyapi implements the exchange of the verification code
// (short term token) for a long term token that can be used to get a
// verification certification to send to the key server.
//
// This is steps 4/5 as specified here:
// https://developers.google.com/android/exposure-notifications/verification-system#flow-diagram
package verifyapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	enobs "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/golang-jwt/jwt"
)

func (c *Controller) HandleVerify() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.config.MaintenanceMode {
			c.h.RenderJSON(w, http.StatusTooManyRequests,
				api.Errorf("server is read-only for maintenance").WithCode(api.ErrMaintenanceMode))
			return
		}

		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("verifyapi.HandleVerify")

		now := time.Now().UTC()

		blame := enobs.BlameNone
		result := enobs.ResultOK
		defer enobs.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &blame)

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			blame = enobs.BlameClient
			result = enobs.ResultError("MISSING_AUTHORIZED_APP")
			logger.Debugw("no authorized app detected", "blame", blame, "result", result)
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		var request api.VerifyCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("bad request", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("FAILED_TO_PARSE_JSON_REQUEST")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}

		// Get the currently active key.
		activeTokenSigningKey, err := c.db.ActiveTokenSigningKeyCached(ctx, c.cacher)
		if err != nil {
			logger.Errorw("failed to get active token signing key", "error", err)
			blame = enobs.BlameServer
			result = enobs.ResultError("FAILED_TO_GET_ACTIVE_TOKEN_SIGNING_KEY")

			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			return
		}

		// Get the signer based on the key configuration.
		signer, err := c.kms.NewSigner(ctx, activeTokenSigningKey.KeyVersionID)
		if err != nil {
			logger.Errorw("failed to get signer", "error", err)
			blame = enobs.BlameServer
			result = enobs.ResultError("FAILED_TO_GET_SIGNER")

			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			return
		}

		// Process and validate the requested acceptable test types.
		acceptTypes, err := request.GetAcceptedTestTypes()
		if err != nil {
			logger.Errorf("invalid accept test types", "error", err)
			blame = enobs.BlameClient
			result = enobs.ResultError("INVALID_ACCEPT_TEST_TYPES")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInvalidTestType))
			return
		}

		nonce := []byte{}
		if request.Nonce != "" {
			nonce, err = base64util.DecodeString(request.Nonce)
			if err != nil {
				blame = enobs.BlameClient
				result = enobs.ResultError("BAD_NONCE")
				logger.Errorw("bad request", "error", err, "blame", blame, "result", result)
				c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
				return
			}
		}

		tokenRequest := &database.IssueTokenRequest{
			Time:        now,
			AuthApp:     authApp,
			VerCode:     request.VerificationCode,
			AcceptTypes: acceptTypes,
			ExpireAfter: c.config.VerificationTokenDuration,
			Nonce:       nonce,
			OS:          controller.OperatingSystemFromContext(ctx),
		}
		// Exchange the short term verification code for a long term verification token.
		// The token can be used to sign TEKs later.
		verificationToken, err := c.db.VerifyCodeAndIssueToken(tokenRequest)
		if err != nil {
			blame = enobs.BlameClient
			switch {
			case errors.Is(err, database.ErrVerificationCodeExpired):
				result = enobs.ResultError("VERIFICATION_CODE_EXPIRED")
				apiErr := api.Errorf("verification code expired").WithCode(api.ErrVerifyCodeExpired)
				logger.Debugw("verify failed: verification code expired", "error", err, "api-error", apiErr)
				c.h.RenderJSON(w, http.StatusBadRequest, apiErr)
				return
			case errors.Is(err, database.ErrVerificationCodeUsed):
				result = enobs.ResultError("VERIFICATION_CODE_INVALID")
				apiErr := api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid)
				logger.Debugw("verify failed: verification code invalid", "error", err, "api-error", apiErr)
				c.h.RenderJSON(w, http.StatusBadRequest, apiErr)
				return
			case errors.Is(err, database.ErrVerificationCodeNotFound):
				result = enobs.ResultError("VERIFICATION_CODE_NOT_FOUND")
				apiErr := api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid)
				logger.Debugw("verify failed: verification code not found", "error", err, "api-error", apiErr)
				c.h.RenderJSON(w, http.StatusBadRequest, apiErr)
				return
			case errors.Is(err, database.ErrUnsupportedTestType):
				result = enobs.ResultError("VERIFICATION_CODE_UNSUPPORTED_TEST_TYPE")
				apiErr := api.Errorf("verification code has unsupported test type").WithCode(api.ErrUnsupportedTestType)
				logger.Debugw("verify failed: unsupported test type", "error", err, "api-error", apiErr)
				c.h.RenderJSON(w, http.StatusPreconditionFailed, apiErr)
				return
			default:
				logger.Errorw("failed to issue verification token", "error", err)
				result = enobs.ResultError("UNKNOWN_ERROR")
				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
				return
			}
		}

		subject := verificationToken.Subject()
		claims := &jwt.StandardClaims{
			Audience:  c.config.TokenSigning.TokenIssuer,
			ExpiresAt: now.Add(c.config.VerificationTokenDuration).Unix(),
			Id:        verificationToken.TokenID,
			IssuedAt:  now.Unix(),
			Issuer:    c.config.TokenSigning.TokenIssuer,
			Subject:   subject.String(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

		// Set the JWT kid to the database record ID. We will use this to lookup the
		// appropriate record to verify.
		token.Header[verifyapi.KeyIDHeader] = activeTokenSigningKey.UUID

		signedJWT, err := jwthelper.SignJWT(token, signer)
		if err != nil {
			logger.Errorw("failed to sign token", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			blame = enobs.BlameServer
			result = enobs.ResultError("FAILED_TO_SIGN_TOKEN")
			return
		}

		c.h.RenderJSON(w, http.StatusOK, api.VerifyCodeResponse{
			TestType:          verificationToken.TestType,
			SymptomDate:       verificationToken.FormatSymptomDate(),
			TestDate:          verificationToken.FormatTestDate(),
			VerificationToken: signedJWT,
		})
	})
}
