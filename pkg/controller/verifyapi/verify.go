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

	"github.com/google/exposure-notifications-verification-server/pkg/api"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
	"github.com/google/exposure-notifications-verification-server/pkg/jwthelper"
	"github.com/google/exposure-notifications-verification-server/pkg/observability"

	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"github.com/dgrijalva/jwt-go"
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

		var blame = observability.BlameNone
		var result = observability.ResultOK()
		defer observability.RecordLatency(ctx, time.Now(), mLatencyMs, &result, &blame)

		authApp := controller.AuthorizedAppFromContext(ctx)
		if authApp == nil {
			blame = observability.BlameClient
			result = observability.ResultError("MISSING_AUTHORIZED_APP")
			controller.MissingAuthorizedApp(w, r, c.h)
			return
		}

		var request api.VerifyCodeRequest
		if err := controller.BindJSON(w, r, &request); err != nil {
			logger.Errorw("bad request", "error", err)
			blame = observability.BlameClient
			result = observability.ResultError("FAILED_TO_PARSE_JSON_REQUEST")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrUnparsableRequest))
			return
		}

		// TODO(mikehelmick|sethvargo) - remove the fallback code after
		fallbackToLgecyKey := true
		legacyKey := c.config.TokenSigning.TokenSigningKeys[0]
		//lint:ignore SA1019 will removed in next release.
		legacyKID := c.config.TokenSigning.TokenSigningKeyIDs[0]

		// Get the currently active key.
		activeTokenSigningKey, err := c.db.ActiveTokenSigningKeyCached(ctx, c.cacher)
		if err != nil {
			if database.IsNotFound(err) {
				logger.Errorw("no token signing key in database, falling back to legacy signing key", "error", err)
			} else {
				logger.Errorw("failed to get active token signing key", "error", err)
				blame = observability.BlameServer
				result = observability.ResultError("FAILED_TO_GET_ACTIVE_TOKEN_SIGNING_KEY")

				c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
				return
			}
		} else {
			// No need to fallback to legacy signing, the key was loaded from the database.
			fallbackToLgecyKey = false
		}

		// Get the signer based on the key configuration.
		keyRef := ""
		if fallbackToLgecyKey {
			keyRef = legacyKey
		} else {
			keyRef = activeTokenSigningKey.KeyVersionID
		}
		signer, err := c.kms.NewSigner(ctx, keyRef)
		if err != nil {
			logger.Errorw("failed to get signer", "error", err)
			blame = observability.BlameServer
			result = observability.ResultError("FAILED_TO_GET_SIGNER")

			c.h.RenderJSON(w, http.StatusInternalServerError, api.InternalError())
			return
		}

		// Process and validate the requested acceptable test types.
		acceptTypes, err := request.GetAcceptedTestTypes()
		if err != nil {
			logger.Errorf("invalid accept test types", "error", err)
			blame = observability.BlameClient
			result = observability.ResultError("INVALID_ACCEPT_TEST_TYPES")

			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInvalidTestType))
			return
		}

		// Exchange the short term verification code for a long term verification token.
		// The token can be used to sign TEKs later.
		verificationToken, err := c.db.VerifyCodeAndIssueToken(now, authApp, request.VerificationCode, acceptTypes, c.config.VerificationTokenDuration)
		if err != nil {
			blame = observability.BlameClient
			switch {
			case errors.Is(err, database.ErrVerificationCodeExpired):
				result = observability.ResultError("VERIFICATION_CODE_EXPIRED")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code expired").WithCode(api.ErrVerifyCodeExpired))
				return
			case errors.Is(err, database.ErrVerificationCodeUsed):
				result = observability.ResultError("VERIFICATION_CODE_INVALID")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
				return
			case errors.Is(err, database.ErrVerificationCodeNotFound):
				result = observability.ResultError("VERIFICATION_CODE_NOT_FOUND")
				c.h.RenderJSON(w, http.StatusBadRequest, api.Errorf("verification code invalid").WithCode(api.ErrVerifyCodeInvalid))
				return
			case errors.Is(err, database.ErrUnsupportedTestType):
				result = observability.ResultError("VERIFICATION_CODE_UNSUPPORTED_TEST_TYPE")
				c.h.RenderJSON(w, http.StatusPreconditionFailed, api.Errorf("verification code has unsupported test type").WithCode(api.ErrUnsupportedTestType))
				return
			default:
				logger.Errorw("failed to issue verification token", "error", err)
				result = observability.ResultError("UNKNOWN_ERROR")
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
		if fallbackToLgecyKey {
			token.Header[verifyapi.KeyIDHeader] = legacyKID
		} else {
			token.Header[verifyapi.KeyIDHeader] = activeTokenSigningKey.UUID
		}

		signedJWT, err := jwthelper.SignJWT(token, signer)
		if err != nil {
			logger.Errorw("failed to sign token", "error", err)
			c.h.RenderJSON(w, http.StatusBadRequest, api.Error(err).WithCode(api.ErrInternal))
			blame = observability.BlameServer
			result = observability.ResultError("FAILED_TO_SIGN_TOKEN")
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
